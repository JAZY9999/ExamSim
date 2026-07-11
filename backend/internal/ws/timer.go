// Package ws implémente le chronomètre synchronisé en temps réel entre
// l'examinateur et l'étudiant pendant l'épreuve orale (« timer f2f »).
//
// Modèle : une "salle" (room) par session orale. L'examinateur contrôle le
// timer (start/pause/reset) ; le serveur décrémente le temps et diffuse l'état
// à tous les participants connectés (examinateur + étudiant) chaque seconde.
// C'est le cœur technique du projet et le sujet de plusieurs diagrammes de
// séquence du dossier de conception.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// En dev, on autorise toutes les origines (le frontend tourne sur un autre port).
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Message échangé entre client et serveur sur la connexion WebSocket.
// "evaluated" est émis par l'examinateur quand il enregistre la note : le
// serveur le relaie tel quel à tous les participants (l'étudiant voit sa note
// s'afficher en direct).
type Message struct {
	Type    string  `json:"type"`    // "start" | "pause" | "reset" | "state" | "set" | "evaluated"
	Seconds int     `json:"seconds"` // temps restant (pour "state" et "set")
	Running bool    `json:"running"` // le timer tourne-t-il ? (pour "state")
	Note    float64 `json:"note,omitempty"`     // note attribuée (pour "evaluated")
	NoteMax float64 `json:"note_max,omitempty"` // barème total (pour "evaluated")
}

// Room est une salle de timer associée à une session orale.
type Room struct {
	id      string
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
	seconds int  // temps restant
	running bool // le compte à rebours est-il actif ?
}

// Hub gère l'ensemble des salles actives.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

// NewHub crée un hub et lance la boucle de décompte globale (1 tick/seconde).
func NewHub() *Hub {
	h := &Hub{rooms: make(map[string]*Room)}
	go h.tickLoop()
	return h
}

func (h *Hub) getRoom(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	if !ok {
		r = &Room{id: id, clients: make(map[*websocket.Conn]bool)}
		h.rooms[id] = r
	}
	return r
}

// tickLoop décrémente chaque seconde les salles actives et diffuse l'état.
func (h *Hub) tickLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		rooms := make([]*Room, 0, len(h.rooms))
		for _, r := range h.rooms {
			rooms = append(rooms, r)
		}
		h.mu.Unlock()

		for _, r := range rooms {
			r.mu.Lock()
			if r.running && r.seconds > 0 {
				r.seconds--
				if r.seconds == 0 {
					r.running = false // fin du temps
				}
			}
			r.mu.Unlock()
			r.broadcastState()
		}
	}
}

// broadcastState envoie l'état courant du timer à tous les clients de la salle.
func (r *Room) broadcastState() {
	r.mu.Lock()
	msg := Message{Type: "state", Seconds: r.seconds, Running: r.running}
	r.mu.Unlock()
	payload, _ := json.Marshal(msg)
	r.broadcastRaw(payload)
}

// broadcastRaw diffuse une charge utile brute à tous les clients de la salle.
func (r *Room) broadcastRaw(payload []byte) {
	r.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(r.clients))
	for c := range r.clients {
		conns = append(conns, c)
	}
	r.mu.Unlock()

	for _, c := range conns {
		if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
			r.mu.Lock()
			delete(r.clients, c)
			r.mu.Unlock()
			c.Close()
		}
	}
}

// Handler gère la connexion WebSocket d'un participant à une salle.
// Route : /ws/oral/{sessionID}
func (h *Hub) Handler(w http.ResponseWriter, req *http.Request) {
	sessionID := chi.URLParam(req, "sessionID")
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println("upgrade WebSocket:", err)
		return
	}

	room := h.getRoom(sessionID)
	room.mu.Lock()
	room.clients[conn] = true
	room.mu.Unlock()

	// Envoie l'état courant au nouvel arrivant.
	room.broadcastState()

	defer func() {
		room.mu.Lock()
		delete(room.clients, conn)
		room.mu.Unlock()
		conn.Close()
	}()

	// Boucle de lecture : traite les commandes de l'examinateur.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg Message
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		room.mu.Lock()
		switch msg.Type {
		case "start":
			room.running = true
		case "pause":
			room.running = false
		case "reset":
			room.running = false
			room.seconds = msg.Seconds
		case "set": // initialise la durée (ex: 15 min)
			room.seconds = msg.Seconds
		case "evaluated": // fin d'épreuve : on stoppe le chrono
			room.running = false
		}
		room.mu.Unlock()

		if msg.Type == "evaluated" {
			// Relaie l'annonce de la note telle quelle à tous les participants.
			payload, _ := json.Marshal(msg)
			room.broadcastRaw(payload)
		} else {
			// Diffuse immédiatement le nouvel état (réactivité).
			room.broadcastState()
		}
	}
}
