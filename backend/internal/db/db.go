// Package db gère la connexion PostgreSQL, l'application du schéma et le seed
// des données de démonstration.
package db

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Connect ouvre un pool de connexions et applique le schéma. Il réessaie
// quelques fois car, avec Docker Compose, Postgres peut mettre un instant à
// être prêt au premier démarrage.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for i := 0; i < 10; i++ {
		pool, err = pgxpool.New(ctx, url)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
		}
		fmt.Printf("⏳ En attente de PostgreSQL (tentative %d/10)...\n", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connexion à PostgreSQL impossible : %w", err)
	}

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return nil, fmt.Errorf("application du schéma : %w", err)
	}
	fmt.Println("✅ Schéma PostgreSQL appliqué")
	return pool, nil
}
