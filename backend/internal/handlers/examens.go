package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"examsim/internal/middleware"
	"examsim/internal/models"

	"github.com/go-chi/chi/v5"
)

// ListExamens renvoie les examens visibles par l'utilisateur :
//   - personnel (examinateur/admin) : tous les examens + SessionsResume + créateur ;
//   - étudiant : examens accessibles (assignation OK) + dans la fenêtre de dispo
//     + non encore terminés par cet étudiant.
func (h *Handler) ListExamens(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())

	var rows interface{ Next() bool; Scan(...any) error; Close() }
	var err error

	if role == models.RoleEtudiant {
		// Étudiant : accès restreint + masquage post-passage + fenêtre de dispo.
		rows, err = h.DB.Query(r.Context(), `
			SELECT e.id, e.titre, e.description, e.type, e.modalite, e.duree_min,
			       e.tags, e.createur_id, u.prenom || ' ' || u.nom, e.created_at,
			       e.disponible_de, e.disponible_jusqu_a
			FROM examens e
			JOIN utilisateurs u ON u.id = e.createur_id
			WHERE (
			  -- accessible (public ou ciblé pour cet étudiant)
			  NOT EXISTS (SELECT 1 FROM examen_assignations a WHERE a.examen_id = e.id)
			  OR EXISTS (SELECT 1 FROM examen_assignations a
			             WHERE a.examen_id = e.id AND a.utilisateur_id = $1)
			  OR EXISTS (SELECT 1 FROM examen_assignations a
			             JOIN classe_membres cm ON cm.classe_id = a.classe_id
			             WHERE a.examen_id = e.id AND cm.utilisateur_id = $1)
			)
			-- non encore terminé par cet étudiant
			AND NOT EXISTS (
			  SELECT 1 FROM sessions s
			  WHERE s.examen_id = e.id AND s.etudiant_id = $1
			  AND s.statut IN ('terminee','evaluee')
			)
			-- dans la fenêtre de disponibilité (si définie)
			AND (e.disponible_de IS NULL OR e.disponible_de <= now())
			AND (e.disponible_jusqu_a IS NULL OR e.disponible_jusqu_a >= now())
			ORDER BY e.created_at DESC`, uid)
	} else {
		// Staff : tous les examens.
		rows, err = h.DB.Query(r.Context(), `
			SELECT e.id, e.titre, e.description, e.type, e.modalite, e.duree_min,
			       e.tags, e.createur_id, u.prenom || ' ' || u.nom, e.created_at,
			       e.disponible_de, e.disponible_jusqu_a
			FROM examens e
			JOIN utilisateurs u ON u.id = e.createur_id
			ORDER BY e.created_at DESC`)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture des examens impossible")
		return
	}

	examens := []models.Examen{}
	for rows.Next() {
		var e models.Examen
		if err := rows.Scan(&e.ID, &e.Titre, &e.Description, &e.Type, &e.Modalite,
			&e.DureeMin, &e.Tags, &e.CreateurID, &e.CreateurNom, &e.CreatedAt,
			&e.DisponibleDe, &e.DisponibleJusqua); err != nil {
			writeError(w, http.StatusInternalServerError, "lecture des examens impossible")
			return
		}
		examens = append(examens, e)
	}
	rows.Close()

	// Enrichissement staff : assignations + SessionsResume.
	for i := range examens {
		a := h.loadAssignations(r.Context(), examens[i].ID)
		if a != nil && (len(a.Classes) > 0 || len(a.Etudiants) > 0) {
			examens[i].Assignations = a
		}
		if role != models.RoleEtudiant {
			examens[i].SessionsResume = h.loadSessionsResume(r.Context(), examens[i].ID)
		}
	}

	writeJSON(w, http.StatusOK, examens)
}

// loadAssignations charge les classes et étudiants ciblés par un examen.
func (h *Handler) loadAssignations(ctx context.Context, examenID string) *models.AssignationResume {
	a := &models.AssignationResume{
		Classes:   []models.NomID{},
		Etudiants: []models.NomID{},
	}
	cRows, err := h.DB.Query(ctx, `
		SELECT c.id, c.nom
		FROM examen_assignations ea JOIN classes c ON c.id = ea.classe_id
		WHERE ea.examen_id = $1 AND ea.classe_id IS NOT NULL
		ORDER BY c.nom`, examenID)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var n models.NomID
			if err := cRows.Scan(&n.ID, &n.Nom); err == nil {
				a.Classes = append(a.Classes, n)
			}
		}
	}
	uRows, err := h.DB.Query(ctx, `
		SELECT u.id, u.prenom || ' ' || u.nom
		FROM examen_assignations ea JOIN utilisateurs u ON u.id = ea.utilisateur_id
		WHERE ea.examen_id = $1 AND ea.utilisateur_id IS NOT NULL
		ORDER BY u.nom, u.prenom`, examenID)
	if err == nil {
		defer uRows.Close()
		for uRows.Next() {
			var n models.NomID
			if err := uRows.Scan(&n.ID, &n.Nom); err == nil {
				a.Etudiants = append(a.Etudiants, n)
			}
		}
	}
	return a
}

// loadSessionsResume calcule la progression des sessions pour un examen ciblé.
// Total = nb d'étudiants uniques assignés (directement ou via classe).
// Terminees = parmi eux, combien ont au moins une session terminée/évaluée.
func (h *Handler) loadSessionsResume(ctx context.Context, examenID string) *models.SessionsResume {
	var total, terminees int
	_ = h.DB.QueryRow(ctx, `
		WITH assignes AS (
		  SELECT DISTINCT utilisateur_id AS uid
		  FROM examen_assignations WHERE examen_id = $1 AND utilisateur_id IS NOT NULL
		  UNION
		  SELECT DISTINCT cm.utilisateur_id
		  FROM examen_assignations ea
		  JOIN classe_membres cm ON cm.classe_id = ea.classe_id
		  WHERE ea.examen_id = $1 AND ea.classe_id IS NOT NULL
		)
		SELECT
		  COUNT(*),
		  COUNT(CASE WHEN EXISTS (
		    SELECT 1 FROM sessions s
		    WHERE s.examen_id = $1 AND s.etudiant_id = assignes.uid
		    AND s.statut IN ('terminee','evaluee')
		  ) THEN 1 END)
		FROM assignes`, examenID).Scan(&total, &terminees)

	if total == 0 {
		return nil // pas de ciblage → pas de résumé
	}
	return &models.SessionsResume{
		Total:        total,
		Terminees:    terminees,
		TousTermines: total > 0 && terminees == total,
	}
}

// GetExamen renvoie un examen avec ses questions et sa grille éventuelle.
func (h *Handler) GetExamen(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.loadExamen(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *Handler) loadExamen(ctx context.Context, id string) (models.Examen, error) {
	var e models.Examen
	err := h.DB.QueryRow(ctx, `
		SELECT e.id, e.titre, e.description, e.type, e.modalite, e.duree_min,
		       e.tags, e.createur_id, u.prenom || ' ' || u.nom, e.created_at,
		       e.disponible_de, e.disponible_jusqu_a
		FROM examens e JOIN utilisateurs u ON u.id = e.createur_id
		WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.Titre, &e.Description, &e.Type, &e.Modalite,
		&e.DureeMin, &e.Tags, &e.CreateurID, &e.CreateurNom, &e.CreatedAt,
		&e.DisponibleDe, &e.DisponibleJusqua)
	if err != nil {
		return e, err
	}
	qRows, err := h.DB.Query(ctx, `
		SELECT id, examen_id, enonce, type, propositions, bonne_reponse, points, ordre
		FROM questions WHERE examen_id = $1 ORDER BY ordre`, id)
	if err == nil {
		defer qRows.Close()
		for qRows.Next() {
			var q models.Question
			if err := qRows.Scan(&q.ID, &q.ExamenID, &q.Enonce, &q.Type,
				&q.Propositions, &q.BonneReponse, &q.Points, &q.Ordre); err == nil {
				e.Questions = append(e.Questions, q)
			}
		}
	}
	e.Grille = h.loadGrille(ctx, id)
	return e, nil
}

func (h *Handler) loadGrille(ctx context.Context, examenID string) *models.GrilleCorrection {
	var g models.GrilleCorrection
	err := h.DB.QueryRow(ctx, `
		SELECT id, examen_id, titre FROM grilles WHERE examen_id = $1`, examenID,
	).Scan(&g.ID, &g.ExamenID, &g.Titre)
	if err != nil {
		return nil
	}
	rows, err := h.DB.Query(ctx, `
		SELECT id, grille_id, libelle, points_max, ordre
		FROM criteres WHERE grille_id = $1 ORDER BY ordre`, g.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var c models.Critere
			if err := rows.Scan(&c.ID, &c.GrilleID, &c.Libelle, &c.PointsMax, &c.Ordre); err == nil {
				g.Criteres = append(g.Criteres, c)
			}
		}
	}
	return &g
}

type createCritereReq struct {
	Libelle   string  `json:"libelle"`
	PointsMax float64 `json:"points_max"`
}

type createGrilleReq struct {
	Titre    string             `json:"titre"`
	Criteres []createCritereReq `json:"criteres"`
}

type assignationsReq struct {
	ClasseIDs      []string `json:"classe_ids"`
	UtilisateurIDs []string `json:"utilisateur_ids"`
}

type createExamenReq struct {
	Titre            string            `json:"titre"`
	Description      string            `json:"description"`
	Type             models.TypeExamen `json:"type"`
	Modalite         models.Modalite   `json:"modalite"`
	DureeMin         int               `json:"duree_min"`
	Tags             []string          `json:"tags"`
	Questions        []models.Question `json:"questions"`
	Grille           *createGrilleReq  `json:"grille"`
	Assignations     *assignationsReq  `json:"assignations"`
	DisponibleDe     *time.Time        `json:"disponible_de"`
	DisponibleJusqua *time.Time        `json:"disponible_jusqu_a"`
}

// CreateExamen crée un sujet d'entraînement ou un examen officiel.
// Réservé au personnel (examinateur/admin).
func (h *Handler) CreateExamen(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var req createExamenReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	if req.Titre == "" {
		writeError(w, http.StatusBadRequest, "le titre est requis")
		return
	}
	if req.Type == "" {
		req.Type = models.ExamenEntrainement
	}
	if req.DureeMin == 0 {
		req.DureeMin = 30
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	var examenID string
	err := h.DB.QueryRow(r.Context(), `
		INSERT INTO examens (titre, description, type, modalite, duree_min, tags, createur_id,
		                     disponible_de, disponible_jusqu_a)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		req.Titre, req.Description, req.Type, req.Modalite, req.DureeMin, req.Tags, uid,
		req.DisponibleDe, req.DisponibleJusqua,
	).Scan(&examenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "création de l'examen impossible")
		return
	}

	h.persistQuestions(r.Context(), examenID, req.Questions)
	h.persistGrille(r.Context(), examenID, req.Grille)
	cible := h.persistAssignations(r.Context(), examenID, req.Assignations)

	h.audit(r.Context(), uid, "examen_cree", "examen", examenID,
		req.Titre+" ("+string(req.Modalite)+", "+string(req.Type)+") — "+cible)
	e, _ := h.loadExamen(r.Context(), examenID)
	writeJSON(w, http.StatusCreated, e)
}

// UpdateExamen modifie un examen existant (créateur ou admin uniquement).
func (h *Handler) UpdateExamen(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())

	// Vérifier que l'appelant est le créateur ou un admin.
	var createurID string
	if err := h.DB.QueryRow(r.Context(), `SELECT createur_id FROM examens WHERE id = $1`, id).Scan(&createurID); err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}
	if createurID != uid && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "vous ne pouvez modifier que vos propres examens")
		return
	}

	var req createExamenReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	if req.Titre == "" {
		writeError(w, http.StatusBadRequest, "le titre est requis")
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	_, err := h.DB.Exec(r.Context(), `
		UPDATE examens SET titre=$1, description=$2, type=$3, modalite=$4,
		  duree_min=$5, tags=$6, disponible_de=$7, disponible_jusqu_a=$8
		WHERE id=$9`,
		req.Titre, req.Description, req.Type, req.Modalite, req.DureeMin, req.Tags,
		req.DisponibleDe, req.DisponibleJusqua, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mise à jour impossible")
		return
	}

	// Remplacement complet des questions.
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM questions WHERE examen_id = $1`, id)
	h.persistQuestions(r.Context(), id, req.Questions)

	// Remplacement de la grille.
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM grilles WHERE examen_id = $1`, id)
	h.persistGrille(r.Context(), id, req.Grille)

	// Remplacement des assignations.
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM examen_assignations WHERE examen_id = $1`, id)
	h.persistAssignations(r.Context(), id, req.Assignations)

	h.audit(r.Context(), uid, "examen_modifie", "examen", id, req.Titre)
	e, _ := h.loadExamen(r.Context(), id)
	writeJSON(w, http.StatusOK, e)
}

// DeleteExamen supprime un examen (créateur ou admin uniquement).
func (h *Handler) DeleteExamen(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())

	var createurID, titre string
	if err := h.DB.QueryRow(r.Context(),
		`SELECT createur_id, titre FROM examens WHERE id = $1`, id).Scan(&createurID, &titre); err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}
	if createurID != uid && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "vous ne pouvez supprimer que vos propres examens")
		return
	}

	if _, err := h.DB.Exec(r.Context(), `DELETE FROM examens WHERE id = $1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "suppression impossible")
		return
	}
	h.audit(r.Context(), uid, "examen_supprime", "examen", id, titre)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- helpers internes -------------------------------------------------------

func (h *Handler) persistQuestions(ctx context.Context, examenID string, questions []models.Question) {
	for i, q := range questions {
		if q.Propositions == nil {
			q.Propositions = []string{}
		}
		if q.Points == 0 {
			q.Points = 1
		}
		_, _ = h.DB.Exec(ctx, `
			INSERT INTO questions (examen_id, enonce, type, propositions, bonne_reponse, points, ordre)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			examenID, q.Enonce, q.Type, q.Propositions, q.BonneReponse, q.Points, i)
	}
}

func (h *Handler) persistGrille(ctx context.Context, examenID string, req *createGrilleReq) {
	if req == nil || len(req.Criteres) == 0 {
		return
	}
	titre := strings.TrimSpace(req.Titre)
	if titre == "" {
		titre = "Grille de correction"
	}
	var grilleID string
	if err := h.DB.QueryRow(ctx, `
		INSERT INTO grilles (examen_id, titre) VALUES ($1,$2) RETURNING id`,
		examenID, titre).Scan(&grilleID); err == nil {
		for i, c := range req.Criteres {
			if c.PointsMax <= 0 {
				c.PointsMax = 5
			}
			_, _ = h.DB.Exec(ctx, `
				INSERT INTO criteres (grille_id, libelle, points_max, ordre)
				VALUES ($1,$2,$3,$4)`, grilleID, c.Libelle, c.PointsMax, i)
		}
	}
}

func (h *Handler) persistAssignations(ctx context.Context, examenID string, req *assignationsReq) string {
	if req == nil {
		return "public (tous les étudiants)"
	}
	for _, cid := range req.ClasseIDs {
		_, _ = h.DB.Exec(ctx, `
			INSERT INTO examen_assignations (examen_id, classe_id) VALUES ($1,$2)`, examenID, cid)
	}
	for _, uidCible := range req.UtilisateurIDs {
		_, _ = h.DB.Exec(ctx, `
			INSERT INTO examen_assignations (examen_id, utilisateur_id) VALUES ($1,$2)`, examenID, uidCible)
	}
	if n := len(req.ClasseIDs) + len(req.UtilisateurIDs); n > 0 {
		return fmt.Sprintf("%d classe(s), %d étudiant(s)", len(req.ClasseIDs), len(req.UtilisateurIDs))
	}
	return "public (tous les étudiants)"
}

// StudentAnswerView représente la réponse simplifiée d'un étudiant pour une question.
type StudentAnswerView struct {
	Choix   *int   `json:"choix"`
	Texte   string `json:"texte"`
	Correct *bool  `json:"correct"`
}

// StudentCritereView représente la note d'un étudiant pour un critère d'oral.
type StudentCritereView struct {
	CritereID string  `json:"critere_id"`
	Points    float64 `json:"points"`
	PointsMax float64 `json:"points_max"`
}


// StudentResultView contient les résultats d'un étudiant pour un examen.
type StudentResultView struct {
	SessionID      string                        `json:"session_id"`
	EtudiantNom    string                        `json:"etudiant_nom"`
	EtudiantEmail  string                        `json:"etudiant_email"`
	Statut         string                        `json:"statut"`
	ScoreAuto      *float64                      `json:"score_auto"`
	NoteEvaluation *float64                      `json:"note_evaluation"`
	Reponses       map[string]*StudentAnswerView `json:"reponses"`
	Criteres       map[string]*StudentCritereView `json:"criteres"` // critere_id -> note
}

// ExamResultsView est la réponse globale pour le tableau Socrative.
type ExamResultsView struct {
	Examen    models.Examen       `json:"examen"`
	Questions []models.Question   `json:"questions"`
	Resultats []StudentResultView `json:"resultats"`
}

// GetExamResults renvoie tous les résultats détaillés (style Socrative) pour un examen.
func (h *Handler) GetExamResults(w http.ResponseWriter, r *http.Request) {
	examenID := chi.URLParam(r, "id")

	// 1. Charger l'examen
	examen, err := h.loadExamen(r.Context(), examenID)
	if err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}

	// 2. Charger les sessions et étudiants associés
	rows, err := h.DB.Query(r.Context(), `
		SELECT s.id, s.statut, s.score_auto, u.prenom || ' ' || u.nom, u.email,
		       (SELECT ev.note_totale FROM evaluations ev
		        WHERE ev.session_id = s.id ORDER BY ev.created_at DESC LIMIT 1)
		FROM sessions s
		JOIN utilisateurs u ON u.id = s.etudiant_id
		WHERE s.examen_id = $1
		ORDER BY u.nom, u.prenom`, examenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur lors du chargement des sessions")
		return
	}
	defer rows.Close()

	resultats := []StudentResultView{}
	sessionIDs := []string{}
	resultsMap := make(map[string]*StudentResultView)

	for rows.Next() {
		var rView StudentResultView
		var scoreEval *float64
		if err := rows.Scan(&rView.SessionID, &rView.Statut, &rView.ScoreAuto, &rView.EtudiantNom, &rView.EtudiantEmail, &scoreEval); err == nil {
			rView.NoteEvaluation = scoreEval
			rView.Reponses = make(map[string]*StudentAnswerView)
			rView.Criteres = make(map[string]*StudentCritereView)
			resultats = append(resultats, rView)
			sessionIDs = append(sessionIDs, rView.SessionID)
		}
	}
	rows.Close()

	// Référencer pour insertion rapide
	for i := range resultats {
		resultsMap[resultats[i].SessionID] = &resultats[i]
	}

	if len(sessionIDs) > 0 {
		// 3. Charger toutes les réponses associées à ces sessions
		ansRows, err := h.DB.Query(r.Context(), `
			SELECT session_id, question_id, choix, texte, correct
			FROM reponses
			WHERE session_id = ANY($1)`, sessionIDs)
		if err == nil {
			defer ansRows.Close()
			for ansRows.Next() {
				var sid, qid string
				var ans StudentAnswerView
				if err := ansRows.Scan(&sid, &qid, &ans.Choix, &ans.Texte, &ans.Correct); err == nil {
					if rv, ok := resultsMap[sid]; ok {
						rv.Reponses[qid] = &ans
					}
				}
			}
		}

		// 4. Charger les notes par critères d'évaluation pour l'oral
		critRows, err := h.DB.Query(r.Context(), `
			SELECT ev.session_id, nc.critere_id, nc.points, c.points_max
			FROM notes_critere nc
			JOIN criteres c ON c.id = nc.critere_id
			JOIN evaluations ev ON ev.id = nc.evaluation_id
			WHERE ev.session_id = ANY($1)`, sessionIDs)
		if err == nil {
			defer critRows.Close()
			for critRows.Next() {
				var sid, cid string
				var cView StudentCritereView
				if err := critRows.Scan(&sid, &cid, &cView.Points, &cView.PointsMax); err == nil {
					cView.CritereID = cid
					if rv, ok := resultsMap[sid]; ok {
						rv.Criteres[cid] = &cView
					}
				}
			}
		}
	}

	// S'assurer que e.Questions est peuplé
	questions := examen.Questions
	if questions == nil {
		questions = []models.Question{}
	}

	view := ExamResultsView{
		Examen:    examen,
		Questions: questions,
		Resultats: resultats,
	}

	writeJSON(w, http.StatusOK, view)
}


