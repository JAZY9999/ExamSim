package handlers

import (
	"context"
	"net/http"
	"strings"

	"examsim/internal/middleware"
	"examsim/internal/models"

	"golang.org/x/crypto/bcrypt"
)

type registerReq struct {
	Nom        string      `json:"nom"`
	Prenom     string      `json:"prenom"`
	Email      string      `json:"email"`
	MotDePasse string      `json:"mot_de_passe"`
	Role       models.Role `json:"role"`
}

type authResp struct {
	Token       string             `json:"token"`
	Utilisateur models.Utilisateur `json:"utilisateur"`
}

// Register crée un nouveau compte (User Story admin/étudiant : inscription).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.MotDePasse == "" || req.Nom == "" {
		writeError(w, http.StatusBadRequest, "nom, email et mot de passe requis")
		return
	}
	if len(req.MotDePasse) < 8 {
		writeError(w, http.StatusBadRequest, "le mot de passe doit faire au moins 8 caractères")
		return
	}
	if req.Role != models.RoleEtudiant && req.Role != models.RoleExaminateur && req.Role != models.RoleAdmin {
		req.Role = models.RoleEtudiant
	}

	// Hachage du mot de passe (jamais stocké en clair).
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

	h.audit(r.Context(), u.ID, "inscription", "utilisateur", u.ID, u.Email+" ("+string(u.Role)+")")
	token, _ := middleware.GenerateToken(h.JWTSecret, u.ID, u.Role)
	writeJSON(w, http.StatusCreated, authResp{Token: token, Utilisateur: u})
}

type loginReq struct {
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
}

// Login authentifie un utilisateur et renvoie un JWT.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var u models.Utilisateur
	var hash string
	err := h.DB.QueryRow(r.Context(), `
		SELECT id, nom, prenom, email, mot_de_passe, role, created_at
		FROM utilisateurs WHERE email = $1`, req.Email,
	).Scan(&u.ID, &u.Nom, &u.Prenom, &u.Email, &hash, &u.Role, &u.CreatedAt)
	if err != nil {
		// Trace des tentatives sur des comptes inexistants (acteur inconnu).
		h.audit(r.Context(), "", "connexion_echec", "utilisateur", "", req.Email)
		writeError(w, http.StatusUnauthorized, "email ou mot de passe incorrect")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.MotDePasse)) != nil {
		h.audit(r.Context(), u.ID, "connexion_echec", "utilisateur", u.ID, req.Email)
		writeError(w, http.StatusUnauthorized, "email ou mot de passe incorrect")
		return
	}

	h.audit(r.Context(), u.ID, "connexion", "utilisateur", u.ID, u.Email)
	token, _ := middleware.GenerateToken(h.JWTSecret, u.ID, u.Role)
	writeJSON(w, http.StatusOK, authResp{Token: token, Utilisateur: u})
}

// Me renvoie le profil de l'utilisateur authentifié.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	u, err := h.getUser(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) getUser(ctx context.Context, id string) (models.Utilisateur, error) {
	var u models.Utilisateur
	err := h.DB.QueryRow(ctx, `
		SELECT id, nom, prenom, email, role, created_at
		FROM utilisateurs WHERE id = $1`, id,
	).Scan(&u.ID, &u.Nom, &u.Prenom, &u.Email, &u.Role, &u.CreatedAt)
	return u, err
}
