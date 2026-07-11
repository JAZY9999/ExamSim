// Package config charge la configuration de l'application depuis les variables
// d'environnement, avec des valeurs par défaut adaptées au développement local.
package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JitsiDomain string // domaine du prestataire visio (ex: meet.jit.si)
}

// Load lit la configuration depuis l'environnement.
func Load() Config {
	return Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://examsim:examsim@localhost:5432/examsim?sslmode=disable"),
		JWTSecret:   getenv("JWT_SECRET", "dev-secret-change-me-in-production"),
		// Choix du prestataire visio :
		//  - meet.jit.si exige un modérateur connecté (les anonymes attendent) ;
		//  - meet.ffmuc.net interdit l'intégration en iframe (frame-ancestors).
		// framatalk.org (Framasoft) accepte les salles anonymes ET l'iframe.
		JitsiDomain: getenv("JITSI_DOMAIN", "framatalk.org"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
