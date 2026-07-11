package handlers

import (
	"net/http"
	"strings"

	"examsim/internal/middleware"
	"examsim/internal/models"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// validRole vérifie qu'un rôle fait partie des rôles connus.
func validRole(r models.Role) bool {
	return r == models.RoleEtudiant || r == models.RoleExaminateur || r == models.RoleAdmin
}

// ListUtilisateurs renvoie tous les comptes (réservé à l'administrateur —
// User Story : gérer les comptes utilisateurs).
func (h *Handler) ListUtilisateurs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT id, nom, prenom, email, role, created_at
		FROM utilisateurs ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture des utilisateurs impossible")
		return
	}
	defer rows.Close()

	users := []models.Utilisateur{}
	for rows.Next() {
		var u models.Utilisateur
		if err := rows.Scan(&u.ID, &u.Nom, &u.Prenom, &u.Email, &u.Role, &u.CreatedAt); err == nil {
			users = append(users, u)
		}
	}
	writeJSON(w, http.StatusOK, users)
}

// CreateUtilisateur crée un compte à la main (vue admin) : profs comme élèves.
func (h *Handler) CreateUtilisateur(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Nom == "" {
		writeError(w, http.StatusBadRequest, "nom et email requis")
		return
	}
	if len(req.MotDePasse) < 8 {
		writeError(w, http.StatusBadRequest, "le mot de passe doit faire au moins 8 caractères")
		return
	}
	if !validRole(req.Role) {
		req.Role = models.RoleEtudiant
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.MotDePasse), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur de hachage")
		return
	}

	var u models.Utilisateur
	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO utilisateurs (nom, prenom, email, mot_de_passe, role)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, nom, prenom, email, role, created_at`,
		req.Nom, req.Prenom, req.Email, string(hash), req.Role,
	).Scan(&u.ID, &u.Nom, &u.Prenom, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "un compte existe déjà avec cet email")
			return
		}
		writeError(w, http.StatusInternalServerError, "création du compte impossible")
		return
	}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"compte_cree", "utilisateur", u.ID, u.Email+" ("+string(u.Role)+")")
	writeJSON(w, http.StatusCreated, u)
}

type updateRoleReq struct {
	Role models.Role `json:"role"`
}

// UpdateRole change le rôle (« statut ») d'un compte. L'admin ne peut pas se
// rétrograder lui-même : cela éviterait de laisser la plateforme sans admin.
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == middleware.UserIDFromContext(r.Context()) {
		writeError(w, http.StatusBadRequest, "vous ne pouvez pas modifier votre propre rôle")
		return
	}
	var req updateRoleReq
	if err := decode(r, &req); err != nil || !validRole(req.Role) {
		writeError(w, http.StatusBadRequest, "rôle invalide")
		return
	}
	tag, err := h.DB.Exec(r.Context(),
		`UPDATE utilisateurs SET role = $2 WHERE id = $1`, id, req.Role)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"role_modifie", "utilisateur", id, "nouveau rôle : "+string(req.Role))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type resetPasswordReq struct {
	MotDePasse string `json:"mot_de_passe"`
}

// ResetPassword définit un nouveau mot de passe pour un compte (vue admin).
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req resetPasswordReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	if len(req.MotDePasse) < 8 {
		writeError(w, http.StatusBadRequest, "le mot de passe doit faire au moins 8 caractères")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.MotDePasse), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur de hachage")
		return
	}
	tag, err := h.DB.Exec(r.Context(),
		`UPDATE utilisateurs SET mot_de_passe = $2 WHERE id = $1`, id, string(hash))
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	// On journalise l'action, jamais le mot de passe.
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"mdp_reinitialise", "utilisateur", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteUtilisateur supprime un compte (et, par cascade, ses sessions,
// réponses et évaluations). Un admin ne peut pas supprimer son propre compte.
func (h *Handler) DeleteUtilisateur(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == middleware.UserIDFromContext(r.Context()) {
		writeError(w, http.StatusBadRequest, "vous ne pouvez pas supprimer votre propre compte")
		return
	}
	// On relève l'email AVANT la suppression pour que le journal garde une
	// trace lisible du compte disparu.
	var email string
	_ = h.DB.QueryRow(r.Context(), `SELECT email FROM utilisateurs WHERE id = $1`, id).Scan(&email)

	tag, err := h.DB.Exec(r.Context(), `DELETE FROM utilisateurs WHERE id = $1`, id)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	h.audit(r.Context(), middleware.UserIDFromContext(r.Context()),
		"compte_supprime", "utilisateur", id, email)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Config expose au frontend la configuration publique (domaine du prestataire
// visio Jitsi utilisé pour l'oral F2F).
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"jitsi_domain": h.JitsiDomain,
	})
}
