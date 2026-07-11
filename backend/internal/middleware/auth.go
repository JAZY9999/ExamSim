// Package middleware fournit l'authentification par JWT et le contrôle d'accès
// par rôle, conformément à la stratégie de sécurité du cahier des charges
// (hachage des mots de passe + protection des routes par token JWT).
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"examsim/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const (
	ctxUserID ctxKey = "user_id"
	ctxRole   ctxKey = "role"
)

// Claims est la charge utile signée dans le JWT.
type Claims struct {
	UserID string      `json:"uid"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken produit un JWT valable 24h pour un utilisateur.
func GenerateToken(secret, userID string, role models.Role) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Authenticator vérifie le token « Authorization: Bearer <jwt> » et injecte
// l'ID utilisateur et le rôle dans le contexte de la requête.
func Authenticator(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "token manquant", http.StatusUnauthorized)
				return
			}
			raw := strings.TrimPrefix(header, "Bearer ")
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "token invalide", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole restreint une route à certains rôles (généralisation d'acteurs).
func RequireRole(roles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "accès interdit pour ce rôle", http.StatusForbidden)
		})
	}
}

// UserIDFromContext récupère l'ID de l'utilisateur authentifié.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

// RoleFromContext récupère le rôle de l'utilisateur authentifié.
func RoleFromContext(ctx context.Context) models.Role {
	if v, ok := ctx.Value(ctxRole).(models.Role); ok {
		return v
	}
	return ""
}
