// Point d'entrée du backend Go de la plateforme de simulation d'examens.
// Il assemble la configuration, la base PostgreSQL, l'authentification JWT,
// les routes REST et le WebSocket du timer oral synchronisé.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"examsim/internal/config"
	"examsim/internal/db"
	"examsim/internal/handlers"
	"examsim/internal/middleware"
	"examsim/internal/models"
	"examsim/internal/ws"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	// Base de données.
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}
	defer pool.Close()
	if err := db.Seed(ctx, pool); err != nil {
		log.Printf("⚠️  seed: %v", err)
	}

	h := handlers.New(pool, cfg.JWTSecret, cfg.JitsiDomain)
	hub := ws.NewHub()

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}))

	// --- Santé & config publique ---
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/api/config", h.Config)

	// --- Routes publiques (authentification) ---
	r.Post("/api/auth/register", h.Register)
	r.Post("/api/auth/login", h.Login)

	// --- Timer oral synchronisé (WebSocket) ---
	r.Get("/ws/oral/{sessionID}", hub.Handler)

	// --- Routes protégées par JWT ---
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg.JWTSecret))

		r.Get("/api/me", h.Me)

		// Examens
		r.Get("/api/examens", h.ListExamens)
		r.Get("/api/examens/{id}", h.GetExamen)

		// Réservé au personnel (examinateur/admin) : création/modification/suppression
		// d'examens et données nécessaires au ciblage (classes, liste des étudiants).
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(models.RoleExaminateur, models.RoleAdmin))
			r.Post("/api/examens", h.CreateExamen)
			r.Patch("/api/examens/{id}", h.UpdateExamen)
			r.Delete("/api/examens/{id}", h.DeleteExamen)
			r.Get("/api/examens/{id}/resultats", h.GetExamResults)
			r.Get("/api/classes", h.ListClasses)
			r.Get("/api/etudiants", h.ListEtudiants)
		})

		// Sessions (passages)
		r.Post("/api/sessions", h.StartSession)
		r.Get("/api/sessions/{id}", h.GetSession)
		r.Get("/api/sessions/{id}/detail", h.GetSessionDetail)
		r.Post("/api/sessions/{id}/submit", h.SubmitSession)
		r.Get("/api/sessions/mine", h.ListMySessions)
		r.Get("/api/sessions/to-evaluate", h.ListSessionsToEvaluate)
		r.Get("/api/sessions/active-orals", h.ListActiveOrals)

		// Évaluations (peer-to-peer & examinateur)
		r.Post("/api/sessions/{id}/evaluations", h.CreateEvaluation)
		r.Get("/api/sessions/{id}/evaluations", h.ListEvaluations)
		r.Get("/api/evaluations/given", h.ListEvaluationsGiven)
		r.Patch("/api/evaluations/{id}", h.UpdateEvaluation)
		r.Patch("/api/evaluations/{id}/visibilite", h.SetNoteVisible)

		// Administration (réservé au rôle admin) : gestion complète des comptes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(models.RoleAdmin))
			r.Get("/api/admin/utilisateurs", h.ListUtilisateurs)
			r.Post("/api/admin/utilisateurs", h.CreateUtilisateur)
			r.Patch("/api/admin/utilisateurs/{id}/role", h.UpdateRole)
			r.Post("/api/admin/utilisateurs/{id}/password", h.ResetPassword)
			r.Delete("/api/admin/utilisateurs/{id}", h.DeleteUtilisateur)
			r.Get("/api/admin/audit", h.ListAuditLog)

			// Gestion des classes
			r.Post("/api/admin/classes", h.CreateClasse)
			r.Delete("/api/admin/classes/{id}", h.DeleteClasse)
			r.Post("/api/admin/classes/{id}/membres", h.AddMembre)
			r.Delete("/api/admin/classes/{id}/membres/{userId}", h.RemoveMembre)
		})
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("🚀 Backend ExamSim démarré sur http://localhost:%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
