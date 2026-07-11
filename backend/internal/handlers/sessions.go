package handlers

import (
	"fmt"
	"net/http"
	"time"

	"examsim/internal/middleware"
	"examsim/internal/models"

	"github.com/go-chi/chi/v5"
)

type startSessionReq struct {
	ExamenID string `json:"examen_id"`
}

// StartSession démarre un passage d'examen pour l'étudiant authentifié
// (User Story : lancer un chronomètre d'entraînement).
func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var req startSessionReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}

	// Durée -> temps restant initial en secondes (+ titre pour le journal).
	var dureeMin int
	var titre string
	if err := h.DB.QueryRow(r.Context(),
		`SELECT duree_min, titre FROM examens WHERE id = $1`, req.ExamenID).Scan(&dureeMin, &titre); err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}

	// Si une session est déjà en cours pour cet examen et cet étudiant, on la
	// réutilise : cela évite de fragmenter l'oral en plusieurs salles (timer
	// et visio doivent réunir tous les participants dans la MÊME session).
	var s models.Session
	err := h.DB.QueryRow(r.Context(), `
		SELECT id, examen_id, etudiant_id, statut, debut_at, temps_restant
		FROM sessions
		WHERE examen_id = $1 AND etudiant_id = $2 AND statut = 'en_cours'
		ORDER BY debut_at DESC LIMIT 1`,
		req.ExamenID, uid,
	).Scan(&s.ID, &s.ExamenID, &s.EtudiantID, &s.Statut, &s.DebutAt, &s.TempsRestant)
	if err == nil {
		writeJSON(w, http.StatusOK, s)
		return
	}
	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO sessions (examen_id, etudiant_id, statut, temps_restant)
		VALUES ($1,$2,'en_cours',$3)
		RETURNING id, examen_id, etudiant_id, statut, debut_at, temps_restant`,
		req.ExamenID, uid, dureeMin*60,
	).Scan(&s.ID, &s.ExamenID, &s.EtudiantID, &s.Statut, &s.DebutAt, &s.TempsRestant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "création de la session impossible")
		return
	}
	h.audit(r.Context(), uid, "session_demarree", "session", s.ID, titre)
	writeJSON(w, http.StatusCreated, s)
}

// GetSession renvoie une session avec l'examen associé (questions + grille)
// et les informations du candidat (affichées côté examinateur pendant l'oral).
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	var s models.Session
	var etuNom, etuPrenom, etuEmail string
	err := h.DB.QueryRow(r.Context(), `
		SELECT s.id, s.examen_id, s.etudiant_id, s.statut, s.debut_at, s.fin_at,
		       s.temps_restant, s.score_auto, u.nom, u.prenom, u.email
		FROM sessions s JOIN utilisateurs u ON u.id = s.etudiant_id
		WHERE s.id = $1`, sessionID,
	).Scan(&s.ID, &s.ExamenID, &s.EtudiantID, &s.Statut, &s.DebutAt, &s.FinAt,
		&s.TempsRestant, &s.ScoreAuto, &etuNom, &etuPrenom, &etuEmail)
	if err != nil {
		writeError(w, http.StatusNotFound, "session introuvable")
		return
	}
	examen, err := h.loadExamen(r.Context(), s.ExamenID)
	if err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session": s,
		"examen":  examen,
		"etudiant": map[string]string{
			"nom": etuNom, "prenom": etuPrenom, "email": etuEmail,
		},
	})
}

// ListActiveOrals renvoie les sessions orales en cours : c'est par cette liste
// que l'examinateur REJOINT la session de l'étudiant (même salle de timer,
// même salle de visio) au lieu d'en créer une nouvelle.
func (h *Handler) ListActiveOrals(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT s.id, s.debut_at, e.titre, e.duree_min, u.prenom, u.nom
		FROM sessions s
		JOIN examens e ON e.id = s.examen_id
		JOIN utilisateurs u ON u.id = s.etudiant_id
		WHERE s.statut = 'en_cours' AND e.modalite = 'oral'
		ORDER BY s.debut_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture impossible")
		return
	}
	defer rows.Close()

	type view struct {
		ID          string    `json:"id"`
		DebutAt     time.Time `json:"debut_at"`
		ExamenTitre string    `json:"examen_titre"`
		DureeMin    int       `json:"duree_min"`
		Etudiant    string    `json:"etudiant"`
	}
	out := []view{}
	for rows.Next() {
		var v view
		var prenom, nom string
		if err := rows.Scan(&v.ID, &v.DebutAt, &v.ExamenTitre, &v.DureeMin, &prenom, &nom); err == nil {
			v.Etudiant = prenom + " " + nom
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GetSessionDetail renvoie la « copie » complète d'une session : les réponses
// question par question, et le détail par critère de chaque évaluation.
// C'est la page de traçabilité centrale de la plateforme.
//
// Autorisations :
//   - le propriétaire de la session (l'étudiant) : toujours ;
//   - examinateur / admin : toujours ;
//   - un autre étudiant (peer-to-peer) : uniquement si la session est terminée
//     ET que l'examen est un sujet d'entraînement.
func (h *Handler) GetSessionDetail(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	uid := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())

	var s models.Session
	var etuNom, etuPrenom, etuEmail string
	err := h.DB.QueryRow(r.Context(), `
		SELECT s.id, s.examen_id, s.etudiant_id, s.statut, s.debut_at, s.fin_at,
		       s.temps_restant, s.score_auto, u.nom, u.prenom, u.email
		FROM sessions s JOIN utilisateurs u ON u.id = s.etudiant_id
		WHERE s.id = $1`, sessionID,
	).Scan(&s.ID, &s.ExamenID, &s.EtudiantID, &s.Statut, &s.DebutAt, &s.FinAt,
		&s.TempsRestant, &s.ScoreAuto, &etuNom, &etuPrenom, &etuEmail)
	if err != nil {
		writeError(w, http.StatusNotFound, "session introuvable")
		return
	}

	examen, err := h.loadExamen(r.Context(), s.ExamenID)
	if err != nil {
		writeError(w, http.StatusNotFound, "examen introuvable")
		return
	}

	isOwner := s.EtudiantID == uid
	isStaff := role == models.RoleExaminateur || role == models.RoleAdmin
	isPeer := !isOwner && !isStaff &&
		s.Statut != models.SessionEnCours && examen.Type == models.ExamenEntrainement
	if !isOwner && !isStaff && !isPeer {
		writeError(w, http.StatusForbidden, "vous n'avez pas accès à cette copie")
		return
	}

	// Réponses de l'étudiant.
	reponses := []models.Reponse{}
	if rRows, err := h.DB.Query(r.Context(), `
		SELECT id, session_id, question_id, choix, texte, correct, created_at
		FROM reponses WHERE session_id = $1`, sessionID); err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var rep models.Reponse
			if err := rRows.Scan(&rep.ID, &rep.SessionID, &rep.QuestionID,
				&rep.Choix, &rep.Texte, &rep.Correct, &rep.CreatedAt); err == nil {
				reponses = append(reponses, rep)
			}
		}
	}

	// Évaluations avec le détail par critère.
	// Pour les étudiants (isOwner) : ne renvoyer que les évaluations avec note_visible=true.
	type noteView struct {
		CritereID string  `json:"critere_id"`
		Libelle   string  `json:"libelle"`
		Points    float64 `json:"points"`
		PointsMax float64 `json:"points_max"`
	}
	type evalView struct {
		ID          string     `json:"id"`
		CorrecteurID string    `json:"correcteur_id"`
		Correcteur  string     `json:"correcteur"`
		Remarques   string     `json:"remarques"`
		NoteTotale  float64    `json:"note_totale"`
		NoteVisible bool       `json:"note_visible"`
		CreatedAt   time.Time  `json:"created_at"`
		Notes       []noteView `json:"notes"`
	}
	evals := []evalView{}
	evalQuery := `
		SELECT ev.id, ev.correcteur_id, COALESCE(u.prenom || ' ' || u.nom, 'Compte supprimé'),
		       ev.remarques, ev.note_totale, ev.note_visible, ev.created_at
		FROM evaluations ev
		LEFT JOIN utilisateurs u ON u.id = ev.correcteur_id
		WHERE ev.session_id = $1`
	// Étudiant propriétaire : seulement les évaluations publiées.
	if isOwner {
		evalQuery += " AND ev.note_visible = true"
	}
	evalQuery += " ORDER BY ev.created_at DESC"
	if eRows, err := h.DB.Query(r.Context(), evalQuery, sessionID); err == nil {
		defer eRows.Close()
		for eRows.Next() {
			var ev evalView
			if err := eRows.Scan(&ev.ID, &ev.CorrecteurID, &ev.Correcteur, &ev.Remarques,
				&ev.NoteTotale, &ev.NoteVisible, &ev.CreatedAt); err != nil {
				continue
			}
			ev.Notes = []noteView{}
			if nRows, err := h.DB.Query(r.Context(), `
				SELECT nc.critere_id, c.libelle, nc.points, c.points_max
				FROM notes_critere nc JOIN criteres c ON c.id = nc.critere_id
				WHERE nc.evaluation_id = $1 ORDER BY c.ordre`, ev.ID); err == nil {
				for nRows.Next() {
					var n noteView
					if err := nRows.Scan(&n.CritereID, &n.Libelle, &n.Points, &n.PointsMax); err == nil {
						ev.Notes = append(ev.Notes, n)
					}
				}
				nRows.Close()
			}
			evals = append(evals, ev)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session": s,
		"examen":  examen,
		"etudiant": map[string]string{
			"nom": etuNom, "prenom": etuPrenom, "email": etuEmail,
		},
		"reponses":     reponses,
		"evaluations":  evals,
		"peut_evaluer": !isOwner && s.Statut != models.SessionEnCours,
		"is_staff":     isStaff,
	})
}


type submitReq struct {
	Reponses []models.Reponse `json:"reponses"`
}

// SubmitSession enregistre les réponses, corrige automatiquement les QCM et
// clôture la session (User Story : soumettre l'examen).
func (h *Handler) SubmitSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	uid := middleware.UserIDFromContext(r.Context())

	// Vérifie que la session appartient bien à l'étudiant.
	var owner string
	if err := h.DB.QueryRow(r.Context(),
		`SELECT etudiant_id FROM sessions WHERE id = $1`, sessionID).Scan(&owner); err != nil {
		writeError(w, http.StatusNotFound, "session introuvable")
		return
	}
	if owner != uid {
		writeError(w, http.StatusForbidden, "cette session ne vous appartient pas")
		return
	}

	var req submitReq
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}

	var scoreObtenu, scoreMax float64
	for _, rep := range req.Reponses {
		// Récupère la bonne réponse et les points de la question (correction auto QCM).
		var bonne *int
		var pts float64
		var qType models.TypeQuestion
		err := h.DB.QueryRow(r.Context(),
			`SELECT bonne_reponse, points, type FROM questions WHERE id = $1`,
			rep.QuestionID).Scan(&bonne, &pts, &qType)
		if err != nil {
			continue
		}

		var correct *bool
		if qType == models.QuestionQCM && bonne != nil && rep.Choix != nil {
			ok := *rep.Choix == *bonne
			correct = &ok
			scoreMax += pts
			if ok {
				scoreObtenu += pts
			}
		}

		_, _ = h.DB.Exec(r.Context(), `
			INSERT INTO reponses (session_id, question_id, choix, texte, correct)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (session_id, question_id)
			DO UPDATE SET choix = EXCLUDED.choix, texte = EXCLUDED.texte, correct = EXCLUDED.correct`,
			sessionID, rep.QuestionID, rep.Choix, rep.Texte, correct)
	}

	// Score sur 20 si des QCM ont été corrigés.
	var score *float64
	if scoreMax > 0 {
		s := scoreObtenu / scoreMax * 20
		score = &s
	}

	_, _ = h.DB.Exec(r.Context(), `
		UPDATE sessions SET statut = 'terminee', fin_at = now(), score_auto = $2
		WHERE id = $1`, sessionID, score)

	detail := "sans score automatique"
	if score != nil {
		detail = fmt.Sprintf("score auto %.1f/20", *score)
	}
	h.audit(r.Context(), uid, "session_soumise", "session", sessionID, detail)

	resp := map[string]any{"statut": "terminee", "score_auto": score}
	writeJSON(w, http.StatusOK, resp)
}

// ListMySessions renvoie l'historique des sessions de l'étudiant (statistiques).
func (h *Handler) ListMySessions(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	// note_eval : dernière note d'évaluation (oral/peer-to-peer) et son barème
	// total, pour compléter le score automatique des QCM.
	rows, err := h.DB.Query(r.Context(), `
		SELECT s.id, s.examen_id, s.etudiant_id, s.statut, s.debut_at, s.fin_at,
		       s.temps_restant, s.score_auto, e.titre,
		       (SELECT ev.note_totale FROM evaluations ev
		        WHERE ev.session_id = s.id ORDER BY ev.created_at DESC LIMIT 1),
		       (SELECT SUM(c.points_max) FROM grilles g
		        JOIN criteres c ON c.grille_id = g.id
		        WHERE g.examen_id = s.examen_id)
		FROM sessions s JOIN examens e ON e.id = s.examen_id
		WHERE s.etudiant_id = $1 ORDER BY s.debut_at DESC`, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture des sessions impossible")
		return
	}
	defer rows.Close()

	type sessionView struct {
		models.Session
		ExamenTitre string   `json:"examen_titre"`
		NoteEval    *float64 `json:"note_eval,omitempty"`
		NoteMax     *float64 `json:"note_max,omitempty"`
	}
	out := []sessionView{}
	for rows.Next() {
		var sv sessionView
		if err := rows.Scan(&sv.ID, &sv.ExamenID, &sv.EtudiantID, &sv.Statut,
			&sv.DebutAt, &sv.FinAt, &sv.TempsRestant, &sv.ScoreAuto, &sv.ExamenTitre,
			&sv.NoteEval, &sv.NoteMax); err == nil {
			out = append(out, sv)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// ListSessionsToEvaluate renvoie les sessions terminées d'autres étudiants,
// disponibles pour une évaluation par les pairs (peer-to-peer).
func (h *Handler) ListSessionsToEvaluate(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	rows, err := h.DB.Query(r.Context(), `
		SELECT s.id, s.examen_id, s.statut, e.titre, u.prenom, u.nom
		FROM sessions s
		JOIN examens e ON e.id = s.examen_id
		JOIN utilisateurs u ON u.id = s.etudiant_id
		WHERE s.statut IN ('terminee','evaluee') AND s.etudiant_id <> $1
		ORDER BY s.debut_at DESC`, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lecture impossible")
		return
	}
	defer rows.Close()

	type view struct {
		ID          string `json:"id"`
		ExamenID    string `json:"examen_id"`
		Statut      string `json:"statut"`
		ExamenTitre string `json:"examen_titre"`
		Etudiant    string `json:"etudiant"`
	}
	out := []view{}
	for rows.Next() {
		var v view
		var prenom, nom string
		if err := rows.Scan(&v.ID, &v.ExamenID, &v.Statut, &v.ExamenTitre, &prenom, &nom); err == nil {
			v.Etudiant = prenom + " " + nom
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
