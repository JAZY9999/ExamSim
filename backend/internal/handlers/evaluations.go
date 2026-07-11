package handlers

import (
	"fmt"
	"net/http"
	"time"

	"examsim/internal/middleware"
	"examsim/internal/models"

	"github.com/go-chi/chi/v5"
)

type createEvalReq struct {
	Remarques   string               `json:"remarques"`
	Notes       []models.NoteCritere `json:"notes"`       // points par critère
	NoteTotale  float64              `json:"note_totale"` // si pas de grille
	NoteVisible bool                 `json:"note_visible"`
}

// CreateEvaluation enregistre une correction (peer-to-peer ou examinateur).
func (h *Handler) CreateEvaluation(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	correcteurID := middleware.UserIDFromContext(r.Context())

	var req createEvalReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}

	var total float64
	for _, n := range req.Notes {
		total += n.Points
	}
	if len(req.Notes) == 0 {
		total = req.NoteTotale
	}

	var evalID string
	err := h.DB.QueryRow(r.Context(), `
		INSERT INTO evaluations (session_id, correcteur_id, remarques, note_totale, note_visible)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		sessionID, correcteurID, req.Remarques, total, req.NoteVisible,
	).Scan(&evalID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "création de l'évaluation impossible")
		return
	}

	for _, n := range req.Notes {
		_, _ = h.DB.Exec(r.Context(), `
			INSERT INTO notes_critere (evaluation_id, critere_id, points)
			VALUES ($1,$2,$3)`, evalID, n.CritereID, n.Points)
	}

	_, _ = h.DB.Exec(r.Context(),
		`UPDATE sessions SET statut = 'evaluee' WHERE id = $1`, sessionID)

	h.audit(r.Context(), correcteurID, "evaluation_enregistree", "session", sessionID,
		fmt.Sprintf("note %.1f (visible=%v)", total, req.NoteVisible))

	eval := models.Evaluation{
		ID:           evalID,
		SessionID:    sessionID,
		CorrecteurID: correcteurID,
		Remarques:    req.Remarques,
		NoteTotale:   total,
		NoteVisible:  req.NoteVisible,
		Notes:        req.Notes,
	}
	writeJSON(w, http.StatusCreated, eval)
}

// UpdateEvaluation modifie une évaluation existante (correcteur original uniquement).
func (h *Handler) UpdateEvaluation(w http.ResponseWriter, r *http.Request) {
	evalID := chi.URLParam(r, "id")
	uid := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())

	// Vérifier propriété.
	var correcteurID, sessionID string
	if err := h.DB.QueryRow(r.Context(),
		`SELECT correcteur_id, session_id FROM evaluations WHERE id = $1`, evalID,
	).Scan(&correcteurID, &sessionID); err != nil {
		writeError(w, http.StatusNotFound, "évaluation introuvable")
		return
	}
	if correcteurID != uid && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "vous ne pouvez modifier que vos propres évaluations")
		return
	}

	var req createEvalReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}

	var total float64
	for _, n := range req.Notes {
		total += n.Points
	}
	if len(req.Notes) == 0 {
		total = req.NoteTotale
	}

	_, err := h.DB.Exec(r.Context(), `
		UPDATE evaluations SET remarques=$1, note_totale=$2, note_visible=$3
		WHERE id=$4`,
		req.Remarques, total, req.NoteVisible, evalID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mise à jour impossible")
		return
	}

	// Remplacement des notes par critère.
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM notes_critere WHERE evaluation_id = $1`, evalID)
	for _, n := range req.Notes {
		_, _ = h.DB.Exec(r.Context(), `
			INSERT INTO notes_critere (evaluation_id, critere_id, points)
			VALUES ($1,$2,$3)`, evalID, n.CritereID, n.Points)
	}

	h.audit(r.Context(), uid, "evaluation_modifiee", "evaluation", evalID,
		fmt.Sprintf("note %.1f (visible=%v)", total, req.NoteVisible))

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           evalID,
		"note_totale":  total,
		"note_visible": req.NoteVisible,
		"remarques":    req.Remarques,
	})
}

// SetNoteVisible bascule la visibilité de la note pour un étudiant.
func (h *Handler) SetNoteVisible(w http.ResponseWriter, r *http.Request) {
	evalID := chi.URLParam(r, "id")
	uid := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())

	var correcteurID string
	if err := h.DB.QueryRow(r.Context(),
		`SELECT correcteur_id FROM evaluations WHERE id = $1`, evalID,
	).Scan(&correcteurID); err != nil {
		writeError(w, http.StatusNotFound, "évaluation introuvable")
		return
	}
	if correcteurID != uid && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "accès refusé")
		return
	}

	var req struct {
		Visible bool `json:"visible"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps invalide")
		return
	}

	_, _ = h.DB.Exec(r.Context(),
		`UPDATE evaluations SET note_visible=$1 WHERE id=$2`, req.Visible, evalID)

	h.audit(r.Context(), uid, "note_visibilite", "evaluation", evalID,
		fmt.Sprintf("visible=%v", req.Visible))
	writeJSON(w, http.StatusOK, map[string]bool{"visible": req.Visible})
}

// ListEvaluationsGiven renvoie l'historique des évaluations réalisées par le correcteur.
func (h *Handler) ListEvaluationsGiven(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	rows, err := h.DB.Query(r.Context(), `
		SELECT ev.id, ev.session_id, ev.note_totale, ev.remarques, ev.created_at,
		       e.titre, u.prenom, u.nom,
		       (SELECT SUM(c.points_max) FROM grilles g
		        JOIN criteres c ON c.grille_id = g.id
		        WHERE g.examen_id = s.examen_id)
		FROM evaluations ev
		JOIN sessions s ON s.id = ev.session_id
		JOIN examens e ON e.id = s.examen_id
		JOIN utilisateurs u ON u.id = s.etudiant_id
		WHERE ev.correcteur_id = $1
		ORDER BY ev.created_at DESC`, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture impossible")
		return
	}
	defer rows.Close()

	type view struct {
		ID          string    `json:"id"`
		SessionID   string    `json:"session_id"`
		NoteTotale  float64   `json:"note_totale"`
		Remarques   string    `json:"remarques"`
		CreatedAt   time.Time `json:"created_at"`
		ExamenTitre string    `json:"examen_titre"`
		Etudiant    string    `json:"etudiant"`
		NoteMax     *float64  `json:"note_max,omitempty"`
	}
	out := []view{}
	for rows.Next() {
		var v view
		var prenom, nom string
		if err := rows.Scan(&v.ID, &v.SessionID, &v.NoteTotale, &v.Remarques,
			&v.CreatedAt, &v.ExamenTitre, &prenom, &nom, &v.NoteMax); err == nil {
			v.Etudiant = prenom + " " + nom
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// ListEvaluations renvoie les évaluations d'une session.
func (h *Handler) ListEvaluations(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	rows, err := h.DB.Query(r.Context(), `
		SELECT e.id, e.session_id, e.correcteur_id, e.remarques, e.note_totale,
		       e.note_visible, e.created_at, u.prenom, u.nom
		FROM evaluations e JOIN utilisateurs u ON u.id = e.correcteur_id
		WHERE e.session_id = $1 ORDER BY e.created_at DESC`, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture impossible")
		return
	}
	defer rows.Close()

	type view struct {
		models.Evaluation
		Correcteur string `json:"correcteur"`
	}
	out := []view{}
	for rows.Next() {
		var v view
		var prenom, nom string
		if err := rows.Scan(&v.ID, &v.SessionID, &v.CorrecteurID, &v.Remarques,
			&v.NoteTotale, &v.NoteVisible, &v.CreatedAt, &prenom, &nom); err == nil {
			v.Correcteur = prenom + " " + nom
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
