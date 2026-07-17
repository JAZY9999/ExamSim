package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Seed insère un jeu de données de démonstration si la base est vide :
// plusieurs étudiants, plusieurs examens (QCM / cas pratique / oral), des
// sessions déjà terminées et évaluées (pour illustrer les résultats et la
// traçabilité), ainsi qu'une session orale EN COURS (pour illustrer le
// timer synchronisé et le rejoin examinateur).
//
// Comptes créés (mot de passe identique : "password") :
//   - admin@examsim.fr        (admin)
//   - prof@examsim.fr         (examinateur)
//   - etudiant@examsim.fr     (étudiant)
//   - camarade@examsim.fr     (étudiant, pour le peer-to-peer)
//   - + 5 étudiants supplémentaires (voir studentSeeds ci-dessous)
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
		{"Lefevre", "Sophie", "prof2@examsim.fr", "examinateur"},
		{"Petit", "Chloé", "etudiant@examsim.fr", "etudiant"},
		{"Roux", "David", "camarade@examsim.fr", "etudiant"},
		{"Bernard", "Emma", "emma.bernard@examsim.fr", "etudiant"},
		{"Moreau", "Lucas", "lucas.moreau@examsim.fr", "etudiant"},
		{"Simon", "Léa", "lea.simon@examsim.fr", "etudiant"},
		{"Laurent", "Hugo", "hugo.laurent@examsim.fr", "etudiant"},
		{"Michel", "Manon", "manon.michel@examsim.fr", "etudiant"},
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
	prof2ID := ids["prof2@examsim.fr"]

	etudiantEmails := []string{
		"etudiant@examsim.fr", "camarade@examsim.fr", "emma.bernard@examsim.fr",
		"lucas.moreau@examsim.fr", "lea.simon@examsim.fr", "hugo.laurent@examsim.fr",
		"manon.michel@examsim.fr",
	}

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
	qcmQuestions := []struct {
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
	var qcmQuestionIDs []string
	for i, q := range qcmQuestions {
		var qid string
		if err := pool.QueryRow(ctx, `
			INSERT INTO questions (examen_id, enonce, type, propositions, bonne_reponse, points, ordre)
			VALUES ($1,$2,'qcm',$3,$4,$5,$6) RETURNING id`,
			qcmID, q.enonce, q.props, q.bonne, q.pts, i).Scan(&qid); err != nil {
			return err
		}
		qcmQuestionIDs = append(qcmQuestionIDs, qid)
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
	var casQuestionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO questions (examen_id, enonce, type, points, ordre)
		VALUES ($1,$2,'cas_pratique',20,0) RETURNING id`,
		casID, "Décrivez en pseudo-code un tri par insertion et donnez sa complexité.").Scan(&casQuestionID); err != nil {
		return err
	}

	// --- Examen 3 : QCM officiel (Bases de données), avec fenêtre de disponibilité ---
	var sqlID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO examens (titre, description, type, modalite, duree_min, tags, createur_id, disponible_de, disponible_jusqu_a)
		VALUES ($1,$2,'officiel','qcm',30,$3,$4,$5,$6) RETURNING id`,
		"Examen SQL Avancé",
		"Jointures, sous-requêtes et transactions.",
		[]string{"SQL", "Bases de données"}, profID,
		time.Now().Add(-48*time.Hour), time.Now().Add(30*24*time.Hour),
	).Scan(&sqlID); err != nil {
		return err
	}
	sqlQuestions := []struct {
		enonce string
		props  []string
		bonne  int
		pts    float64
	}{
		{"Quelle clause filtre les groupes après un GROUP BY ?", []string{"WHERE", "HAVING", "FILTER", "ON"}, 1, 5},
		{"Quel type de jointure conserve les lignes sans correspondance à gauche ?", []string{"INNER JOIN", "RIGHT JOIN", "LEFT JOIN", "CROSS JOIN"}, 2, 5},
		{"Quelle commande garantit l'atomicité d'un ensemble d'opérations ?", []string{"COMMIT seul", "TRANSACTION", "INDEX", "VACUUM"}, 1, 5},
		{"Une clé étrangère assure :", []string{"L'unicité", "L'intégrité référentielle", "Le tri", "La compression"}, 1, 5},
	}
	var sqlQuestionIDs []string
	for i, q := range sqlQuestions {
		var qid string
		if err := pool.QueryRow(ctx, `
			INSERT INTO questions (examen_id, enonce, type, propositions, bonne_reponse, points, ordre)
			VALUES ($1,$2,'qcm',$3,$4,$5,$6) RETURNING id`,
			sqlID, q.enonce, q.props, q.bonne, q.pts, i).Scan(&qid); err != nil {
			return err
		}
		sqlQuestionIDs = append(sqlQuestionIDs, qid)
	}

	// --- Examen 4 : Oral officiel (Soutenance) avec grille de correction ---
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
	var critereIDs []string
	for i, c := range criteres {
		var cid string
		if err := pool.QueryRow(ctx, `
			INSERT INTO criteres (grille_id, libelle, points_max, ordre)
			VALUES ($1,$2,$3,$4) RETURNING id`, grilleID, c.libelle, c.pts, i).Scan(&cid); err != nil {
			return err
		}
		critereIDs = append(critereIDs, cid)
	}

	// --- Examen 5 : Cas pratique officiel (Prof 2), ciblé sur la classe ---
	var casOffID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO examens (titre, description, type, modalite, duree_min, tags, createur_id)
		VALUES ($1,$2,'officiel','cas_pratique',60,$3,$4) RETURNING id`,
		"Étude de cas : Architecture microservices",
		"Analysez et proposez une architecture pour un système distribué.",
		[]string{"Architecture", "Cas pratique"}, prof2ID,
	).Scan(&casOffID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO questions (examen_id, enonce, type, points, ordre)
		VALUES ($1,$2,'cas_pratique',20,0)`,
		casOffID, "Proposez un découpage en microservices pour une plateforme e-commerce et justifiez vos choix."); err != nil {
		return err
	}

	// --- Classe de démonstration avec tous les étudiants ---
	classeID, err := seedClassesReturningID(ctx, pool, etudiantEmails)
	if err != nil {
		return err
	}

	// --- Cibler l'examen SQL et le cas pratique officiel sur la classe ---
	for _, examID := range []string{sqlID, casOffID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO examen_assignations (examen_id, classe_id) VALUES ($1,$2)`,
			examID, classeID); err != nil {
			return err
		}
	}

	// --- Sessions terminées + évaluées sur le QCM Réseaux (correction auto) ---
	// Chaque étudiant répond avec un profil de réussite différent.
	qcmProfiles := map[string][]int{
		"etudiant@examsim.fr":     {1, 1, 2, 1, 2}, // 5/5 bonnes
		"camarade@examsim.fr":     {1, 0, 2, 1, 2}, // 4/5
		"emma.bernard@examsim.fr": {1, 1, 2, 3, 2}, // 4/5
		"lucas.moreau@examsim.fr": {0, 1, 1, 1, 0}, // 1/5
		"lea.simon@examsim.fr":    {1, 1, 2, 1, 2}, // 5/5
		"hugo.laurent@examsim.fr": {1, 2, 2, 1, 1}, // 3/5
	}
	qcmBonnes := []int{1, 1, 2, 1, 2}
	for email, choix := range qcmProfiles {
		etuID := ids[email]
		var scoreObtenu, scoreMax float64
		var sessionID string
		debut := time.Now().Add(-time.Duration(24+len(email)) * time.Hour)
		fin := debut.Add(18 * time.Minute)
		if err := pool.QueryRow(ctx, `
			INSERT INTO sessions (examen_id, etudiant_id, statut, debut_at, fin_at, temps_restant)
			VALUES ($1,$2,'terminee',$3,$4,0) RETURNING id`,
			qcmID, etuID, debut, fin,
		).Scan(&sessionID); err != nil {
			return err
		}
		for i, qid := range qcmQuestionIDs {
			correct := choix[i] == qcmBonnes[i]
			scoreMax += qcmQuestions[i].pts
			if correct {
				scoreObtenu += qcmQuestions[i].pts
			}
			if _, err := pool.Exec(ctx, `
				INSERT INTO reponses (session_id, question_id, choix, correct)
				VALUES ($1,$2,$3,$4)`,
				sessionID, qid, choix[i], correct); err != nil {
				return err
			}
		}
		score := scoreObtenu / scoreMax * 20
		if _, err := pool.Exec(ctx, `
			UPDATE sessions SET score_auto = $2 WHERE id = $1`, sessionID, score); err != nil {
			return err
		}
	}

	// --- Sessions terminées + évaluées sur l'examen SQL (officiel, correction auto) ---
	sqlBonnes := []int{1, 2, 1, 1}
	sqlProfiles := map[string][]int{
		"etudiant@examsim.fr":     {1, 2, 1, 1}, // 4/4
		"emma.bernard@examsim.fr": {1, 2, 0, 1}, // 3/4
		"lea.simon@examsim.fr":    {0, 2, 1, 0}, // 2/4
	}
	for email, choix := range sqlProfiles {
		etuID := ids[email]
		var scoreObtenu, scoreMax float64
		var sessionID string
		debut := time.Now().Add(-time.Duration(10+len(email)) * time.Hour)
		fin := debut.Add(27 * time.Minute)
		if err := pool.QueryRow(ctx, `
			INSERT INTO sessions (examen_id, etudiant_id, statut, debut_at, fin_at, temps_restant)
			VALUES ($1,$2,'terminee',$3,$4,0) RETURNING id`,
			sqlID, etuID, debut, fin,
		).Scan(&sessionID); err != nil {
			return err
		}
		for i, qid := range sqlQuestionIDs {
			correct := choix[i] == sqlBonnes[i]
			scoreMax += sqlQuestions[i].pts
			if correct {
				scoreObtenu += sqlQuestions[i].pts
			}
			if _, err := pool.Exec(ctx, `
				INSERT INTO reponses (session_id, question_id, choix, correct)
				VALUES ($1,$2,$3,$4)`,
				sessionID, qid, choix[i], correct); err != nil {
				return err
			}
		}
		score := scoreObtenu / scoreMax * 20
		if _, err := pool.Exec(ctx, `
			UPDATE sessions SET score_auto = $2 WHERE id = $1`, sessionID, score); err != nil {
			return err
		}
	}

	// --- Sessions terminées + évaluées (par le prof) sur le cas pratique ---
	casCopies := []struct {
		email   string
		texte   string
		note    float64
		remark  string
		visible bool
	}{
		{"etudiant@examsim.fr", "Le tri par insertion parcourt le tableau et insère chaque élément à sa place dans la partie déjà triée. Complexité O(n²) dans le pire cas, O(n) si le tableau est déjà trié.", 18, "Excellent travail ! Les objectifs sont atteints avec brio.", true},
		{"camarade@examsim.fr", "On compare les éléments deux à deux et on les échange. Complexité O(n log n).", 10, "Bonne compréhension globale mais manque de rigueur ou de précision.", true},
		{"lucas.moreau@examsim.fr", "for i in tableau: trier()", 6, "Rendu incomplet, plusieurs questions importantes n'ont pas été traitées.", false},
	}
	for _, c := range casCopies {
		etuID := ids[c.email]
		var sessionID string
		debut := time.Now().Add(-72 * time.Hour)
		fin := debut.Add(40 * time.Minute)
		if err := pool.QueryRow(ctx, `
			INSERT INTO sessions (examen_id, etudiant_id, statut, debut_at, fin_at, temps_restant)
			VALUES ($1,$2,'evaluee',$3,$4,0) RETURNING id`,
			casID, etuID, debut, fin,
		).Scan(&sessionID); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO reponses (session_id, question_id, texte)
			VALUES ($1,$2,$3)`,
			sessionID, casQuestionID, c.texte); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO evaluations (session_id, correcteur_id, remarques, note_totale, note_visible)
			VALUES ($1,$2,$3,$4,$5)`,
			sessionID, profID, c.remark, c.note, c.visible); err != nil {
			return err
		}
	}

	// --- Sessions orales terminées + évaluées (grille complète) ---
	oralCopies := []struct {
		email   string
		notes   []float64 // une par critère
		remark  string
		visible bool
	}{
		{"etudiant@examsim.fr", []float64{5, 4, 5, 4}, "Très bonne soutenance, discours clair et assuré.", true},
		{"emma.bernard@examsim.fr", []float64{4, 3, 4, 5}, "Prestation solide, quelques hésitations techniques.", true},
		{"hugo.laurent@examsim.fr", []float64{3, 2, 3, 4}, "Manque de préparation sur les questions techniques.", false},
	}
	for _, c := range oralCopies {
		etuID := ids[c.email]
		var sessionID string
		debut := time.Now().Add(-5 * 24 * time.Hour)
		fin := debut.Add(15 * time.Minute)
		if err := pool.QueryRow(ctx, `
			INSERT INTO sessions (examen_id, etudiant_id, statut, debut_at, fin_at, temps_restant)
			VALUES ($1,$2,'evaluee',$3,$4,0) RETURNING id`,
			oralID, etuID, debut, fin,
		).Scan(&sessionID); err != nil {
			return err
		}
		var total float64
		var evalID string
		for _, n := range c.notes {
			total += n
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO evaluations (session_id, correcteur_id, remarques, note_totale, note_visible)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			sessionID, profID, c.remark, total, c.visible,
		).Scan(&evalID); err != nil {
			return err
		}
		for i, cid := range critereIDs {
			if _, err := pool.Exec(ctx, `
				INSERT INTO notes_critere (evaluation_id, critere_id, points)
				VALUES ($1,$2,$3)`, evalID, cid, c.notes[i]); err != nil {
				return err
			}
		}
	}

	// --- Session orale EN COURS : illustre le timer synchronisé + le rejoin
	// examinateur depuis « Oraux en cours ». Le candidat est Léa Simon.
	{
		etuID := ids["lea.simon@examsim.fr"]
		if _, err := pool.Exec(ctx, `
			INSERT INTO sessions (examen_id, etudiant_id, statut, debut_at, temps_restant)
			VALUES ($1,$2,'en_cours',now(),$3)`,
			oralID, etuID, 15*60); err != nil {
			return err
		}
	}

	// --- Session QCM EN COURS (entraînement) : illustre un examen non terminé
	// dans les statistiques de l'étudiant principal.
	{
		etuID := ids["manon.michel@examsim.fr"]
		if _, err := pool.Exec(ctx, `
			INSERT INTO sessions (examen_id, etudiant_id, statut, debut_at, temps_restant)
			VALUES ($1,$2,'en_cours',now(),$3)`,
			sqlID, etuID, 25*60); err != nil {
			return err
		}
	}

	fmt.Println("🌱 Données de démonstration insérées : 10 comptes, 5 examens, sessions terminées/évaluées + 1 oral en cours (mdp: password)")
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
	_, err := seedClassesReturningID(ctx, pool, []string{"etudiant@examsim.fr", "camarade@examsim.fr"})
	return err
}

// seedClassesReturningID crée la classe de démonstration avec les emails
// fournis comme membres, et renvoie son id.
func seedClassesReturningID(ctx context.Context, pool *pgxpool.Pool, membreEmails []string) (string, error) {
	var classeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO classes (nom) VALUES ('B2 Informatique') RETURNING id`).Scan(&classeID); err != nil {
		return "", err
	}
	for _, email := range membreEmails {
		_, _ = pool.Exec(ctx, `
			INSERT INTO classe_membres (classe_id, utilisateur_id)
			SELECT $1, id FROM utilisateurs WHERE email = $2
			ON CONFLICT DO NOTHING`, classeID, email)
	}
	fmt.Println("🏫 Classe de démonstration « B2 Informatique » créée")
	return classeID, nil
}
