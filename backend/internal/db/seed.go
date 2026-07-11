package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Seed insère un jeu de données de démonstration si la base est vide.
// Comptes créés (mot de passe identique : "password") :
//   - admin@examsim.fr        (admin)
//   - prof@examsim.fr         (examinateur)
//   - etudiant@examsim.fr     (étudiant)
//   - camarade@examsim.fr     (étudiant, pour le peer-to-peer)
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM utilisateurs`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		// Base déjà peuplée : on ne rejoue pas le seed complet, mais on
		// s'assure que la classe de démo existe (migration douce).
		if err := seedClasses(ctx, pool); err != nil {
			return err
		}
		fmt.Println("ℹ️  Données déjà présentes, seed ignoré")
		return nil
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	pw := string(hash)

	// --- Utilisateurs ---
	users := []struct {
		nom, prenom, email, role string
	}{
		{"Martin", "Alice", "admin@examsim.fr", "admin"},
		{"Dubois", "Bernard", "prof@examsim.fr", "examinateur"},
		{"Petit", "Chloé", "etudiant@examsim.fr", "etudiant"},
		{"Roux", "David", "camarade@examsim.fr", "etudiant"},
	}
	ids := map[string]string{}
	for _, u := range users {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO utilisateurs (nom, prenom, email, mot_de_passe, role)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			u.nom, u.prenom, u.email, pw, u.role).Scan(&id)
		if err != nil {
			return err
		}
		ids[u.email] = id
	}
	profID := ids["prof@examsim.fr"]

	// --- Examen 1 : QCM d'entraînement (Réseaux) ---
	var qcmID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO examens (titre, description, type, modalite, duree_min, tags, createur_id)
		VALUES ($1,$2,'entrainement','qcm',20,$3,$4) RETURNING id`,
		"QCM Réseaux TCP/IP",
		"Testez vos bases sur le modèle OSI et TCP/IP.",
		[]string{"QCM", "Réseaux"}, profID,
	).Scan(&qcmID); err != nil {
		return err
	}
	qcms := []struct {
		enonce string
		props  []string
		bonne  int
		pts    float64
	}{
		{"Combien de couches compte le modèle OSI ?", []string{"5", "7", "4", "6"}, 1, 4},
		{"Quel protocole est orienté connexion ?", []string{"UDP", "TCP", "ICMP", "ARP"}, 1, 4},
		{"Le port HTTP par défaut est :", []string{"21", "25", "80", "443"}, 2, 4},
		{"Quelle couche gère le routage IP ?", []string{"Transport", "Réseau", "Liaison", "Application"}, 1, 4},
		{"DNS sert principalement à :", []string{"Chiffrer", "Router", "Résoudre des noms", "Compresser"}, 2, 4},
	}
	for i, q := range qcms {
		if _, err := pool.Exec(ctx, `
			INSERT INTO questions (examen_id, enonce, type, propositions, bonne_reponse, points, ordre)
			VALUES ($1,$2,'qcm',$3,$4,$5,$6)`,
			qcmID, q.enonce, q.props, q.bonne, q.pts, i); err != nil {
			return err
		}
	}

	// --- Examen 2 : Cas pratique écrit (Algorithmique) ---
	var casID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO examens (titre, description, type, modalite, duree_min, tags, createur_id)
		VALUES ($1,$2,'entrainement','cas_pratique',45,$3,$4) RETURNING id`,
		"Cas pratique : Tri d'un tableau",
		"Rédigez et justifiez un algorithme de tri.",
		[]string{"Cas pratique", "Algorithmique"}, profID,
	).Scan(&casID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO questions (examen_id, enonce, type, points, ordre)
		VALUES ($1,$2,'cas_pratique',20,0)`,
		casID, "Décrivez en pseudo-code un tri par insertion et donnez sa complexité."); err != nil {
		return err
	}

	// --- Examen 3 : Oral officiel (Soutenance) avec grille de correction ---
	var oralID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO examens (titre, description, type, modalite, duree_min, tags, createur_id)
		VALUES ($1,$2,'officiel','oral',15,$3,$4) RETURNING id`,
		"Oral de soutenance projet",
		"Épreuve orale en face-à-face évaluée par un examinateur.",
		[]string{"Oral", "Soutenance"}, profID,
	).Scan(&oralID); err != nil {
		return err
	}
	var grilleID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO grilles (examen_id, titre) VALUES ($1,$2) RETURNING id`,
		oralID, "Grille de soutenance orale").Scan(&grilleID); err != nil {
		return err
	}
	criteres := []struct {
		libelle string
		pts     float64
	}{
		{"Clarté de la présentation", 5},
		{"Maîtrise technique du sujet", 5},
		{"Qualité des réponses aux questions", 5},
		{"Respect du temps imparti", 5},
	}
	for i, c := range criteres {
		if _, err := pool.Exec(ctx, `
			INSERT INTO criteres (grille_id, libelle, points_max, ordre)
			VALUES ($1,$2,$3,$4)`, grilleID, c.libelle, c.pts, i); err != nil {
			return err
		}
	}

	// --- Classe de démonstration avec les deux étudiants ---
	if err := seedClasses(ctx, pool); err != nil {
		return err
	}

	fmt.Println("🌱 Données de démonstration insérées (comptes: admin@ / prof@ / etudiant@ / camarade@examsim.fr, mdp: password)")
	return nil
}

// seedClasses crée la classe de démonstration « B2 Informatique » avec les
// étudiants du seed, si elle n'existe pas déjà. Idempotent : peut être appelée
// sur une base déjà peuplée (migration douce).
func seedClasses(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM classes`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var classeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO classes (nom) VALUES ('B2 Informatique') RETURNING id`).Scan(&classeID); err != nil {
		return err
	}
	for _, email := range []string{"etudiant@examsim.fr", "camarade@examsim.fr"} {
		_, _ = pool.Exec(ctx, `
			INSERT INTO classe_membres (classe_id, utilisateur_id)
			SELECT $1, id FROM utilisateurs WHERE email = $2
			ON CONFLICT DO NOTHING`, classeID, email)
	}
	fmt.Println("🏫 Classe de démonstration « B2 Informatique » créée")
	return nil
}
