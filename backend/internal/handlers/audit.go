package handlers

import (
	"context"
	"net/http"
	"time"
)

// audit enregistre une entrée dans le journal de traçabilité. L'écriture est
// « best effort » : une erreur de journalisation ne doit jamais bloquer
// l'action métier elle-même.
//
// Actions journalisées dans l'application :
//   - connexion / connexion_echec / inscription
//   - examen_cree
//   - session_demarree / session_soumise
//   - evaluation_enregistree
//   - compte_cree / role_modifie / mdp_reinitialise / compte_supprime (admin)
func (h *Handler) audit(ctx context.Context, acteurID, action, cibleType, cibleID, details string) {
	var acteur any
	if acteurID != "" {
		acteur = acteurID // nil si acteur inconnu (ex: échec de connexion)
	}
	_, _ = h.DB.Exec(ctx, `
		INSERT INTO audit_log (acteur_id, action, cible_type, cible_id, details)
		VALUES ($1,$2,$3,$4,$5)`,
		acteur, action, cibleType, cibleID, details)
}

// ListAuditLog renvoie les 300 dernières entrées du journal (vue admin).
// Le nom de l'acteur est résolu au moment de la lecture ; si le compte a été
// supprimé, la trace subsiste avec la mention « Compte supprimé ».
func (h *Handler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT a.id, a.action, a.cible_type, a.cible_id, a.details, a.created_at,
		       COALESCE(u.prenom || ' ' || u.nom,
		                CASE WHEN a.action = 'connexion_echec'
		                     THEN 'Acteur inconnu' ELSE 'Compte supprimé' END),
		       COALESCE(u.email, '')
		FROM audit_log a
		LEFT JOIN utilisateurs u ON u.id = a.acteur_id
		ORDER BY a.created_at DESC
		LIMIT 300`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture du journal impossible")
		return
	}
	defer rows.Close()

	type entry struct {
		ID          string    `json:"id"`
		Action      string    `json:"action"`
		CibleType   string    `json:"cible_type"`
		CibleID     string    `json:"cible_id"`
		Details     string    `json:"details"`
		CreatedAt   time.Time `json:"created_at"`
		Acteur      string    `json:"acteur"`
		ActeurEmail string    `json:"acteur_email"`
	}
	out := []entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.Action, &e.CibleType, &e.CibleID, &e.Details,
			&e.CreatedAt, &e.Acteur, &e.ActeurEmail); err == nil {
			out = append(out, e)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
