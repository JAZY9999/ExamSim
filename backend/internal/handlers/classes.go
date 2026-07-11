package handlers

import (
	"net/http"
	"strings"
	"time"

	"examsim/internal/middleware"
	"examsim/internal/models"

	"github.com/go-chi/chi/v5"
)

// membreView est la représentation légère d'un étudiant dans une classe.
type membreView struct {
	ID     string `json:"id"`
	Prenom string `json:"prenom"`
	Nom    string `json:"nom"`
	Email  string `json:"email"`
}

type classeView struct {
	ID        string       `json:"id"`
	Nom       string       `json:"nom"`
	CreatedAt time.Time    `json:"created_at"`
	Membres   []membreView `json:"membres"`
}

// ListClasses renvoie les classes avec leurs membres. Accessible au personnel
// (examinateur/admin) : sert au ciblage des examens et à la gestion admin.
func (h *Handler) ListClasses(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT id, nom, created_at FROM classes ORDER BY nom`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture des classes impossible")
		return
	}
	defer rows.Close()

	classes := []classeView{}
	for rows.Next() {
		var c classeView
		if err := rows.Scan(&c.ID, &c.Nom, &c.CreatedAt); err == nil {
			c.Membres = []membreView{}
			classes = append(classes, c)
		}
	}
	rows.Close()

	for i := range classes {
		mRows, err := h.DB.Query(r.Context(), `
			SELECT u.id, u.prenom, u.nom, u.email
			FROM classe_membres cm JOIN utilisateurs u ON u.id = cm.utilisateur_id
			WHERE cm.classe_id = $1 ORDER BY u.nom, u.prenom`, classes[i].ID)
		if err != nil {
			continue
		}
		for mRows.Next() {
			var m membreView
			if err := mRows.Scan(&m.ID, &m.Prenom, &m.Nom, &m.Email); err == nil {
				classes[i].Membres = append(classes[i].Membres, m)
			}
		}
		mRows.Close()
	}
	writeJSON(w, http.StatusOK, classes)
}

// ListEtudiants renvoie tous les comptes étudiants (pour le ciblage individuel
// d'un examen et l'ajout aux classes). Réservé au personnel.
func (h *Handler) ListEtudiants(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT id, prenom, nom, email FROM utilisateurs
		WHERE role = 'etudiant' ORDER BY nom, prenom`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture des étudiants impossible")
		return
	}
	defer rows.Close()

	out := []membreView{}
	for rows.Next() {
		var m membreView
		if err := rows.Scan(&m.ID, &m.Prenom, &m.Nom, &m.Email); err == nil {
			out = append(out, m)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type createClasseReq struct {
	Nom string `json:"nom"`
}

// CreateClasse crée une classe (admin).
func (h *Handler) CreateClasse(w http.ResponseWriter, r *http.Request) {
	var req createClasseReq
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.Nom) == "" {
		writeError(w, http.StatusBadRequest, "le nom de la classe est requis")
		return
	}
	req.Nom = strings.TrimSpace(req.Nom)

	var c classeView
	err := h.DB.QueryRow(r.Context(), `
		INSERT INTO classes (nom) VALUES ($1) RETURNING id, nom, created_at`,
		req.Nom).Scan(&c.ID, &c.Nom, &c.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "une classe porte déjà ce nom")
			return
		}
		writeError(w, http.StatusInternalServerError, "création de la classe impossible")
		return
	}
	c.Membres = []membreView{}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"classe_creee", "classe", c.ID, c.Nom)
	writeJSON(w, http.StatusCreated, c)
}

// DeleteClasse supprime une classe (les assignations d'examens liées sont
// supprimées en cascade ; les étudiants eux-mêmes ne sont pas touchés).
func (h *Handler) DeleteClasse(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var nom string
	_ = h.DB.QueryRow(r.Context(), `SELECT nom FROM classes WHERE id = $1`, id).Scan(&nom)

	tag, err := h.DB.Exec(r.Context(), `DELETE FROM classes WHERE id = $1`, id)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "classe introuvable")
		return
	}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"classe_supprimee", "classe", id, nom)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type addMembreReq struct {
	UtilisateurID string `json:"utilisateur_id"`
}

// AddMembre ajoute un étudiant à une classe (admin).
func (h *Handler) AddMembre(w http.ResponseWriter, r *http.Request) {
	classeID := chi.URLParam(r, "id")
	var req addMembreReq
	if err := decode(r, &req); err != nil || req.UtilisateurID == "" {
		writeError(w, http.StatusBadRequest, "utilisateur_id requis")
		return
	}

	// Seuls les étudiants peuvent appartenir à une classe.
	var role models.Role
	if err := h.DB.QueryRow(r.Context(),
		`SELECT role FROM utilisateurs WHERE id = $1`, req.UtilisateurID).Scan(&role); err != nil {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	if role != models.RoleEtudiant {
		writeError(w, http.StatusBadRequest, "seul un étudiant peut être membre d'une classe")
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO classe_membres (classe_id, utilisateur_id)
		VALUES ($1,$2) ON CONFLICT DO NOTHING`, classeID, req.UtilisateurID); err != nil {
		writeError(w, http.StatusNotFound, "classe introuvable")
		return
	}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"membre_ajoute", "classe", classeID, req.UtilisateurID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveMembre retire un étudiant d'une classe (admin).
func (h *Handler) RemoveMembre(w http.ResponseWriter, r *http.Request) {
	classeID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	_, err := h.DB.Exec(r.Context(), `
		DELETE FROM classe_membres WHERE classe_id = $1 AND utilisateur_id = $2`,
		classeID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "retrait impossible")
		return
	}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"membre_retire", "classe", classeID, userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
