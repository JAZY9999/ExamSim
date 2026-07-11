package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler regroupe les dépendances partagées par tous les handlers HTTP.
type Handler struct {
	DB          *pgxpool.Pool
	JWTSecret   string
	JitsiDomain string
}

// New construit le conteneur de handlers.
func New(db *pgxpool.Pool, jwtSecret, jitsiDomain string) *Handler {
	return &Handler{DB: db, JWTSecret: jwtSecret, JitsiDomain: jitsiDomain}
}

// writeJSON sérialise une valeur en JSON avec le code de statut donné.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError renvoie une erreur JSON structurée.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decode lit le corps JSON de la requête dans dst.
func decode(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
