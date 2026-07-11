// Package models regroupe les entités métier de la plateforme de simulation
// d'examens. Ces structures correspondent directement au diagramme de classes
// UML du dossier de conception.
package models

import "time"

// Role définit le type d'utilisateur (acteurs primaires du diagramme de cas
// d'utilisation : Étudiant, Examinateur, Administrateur).
type Role string

const (
	RoleEtudiant     Role = "etudiant"
	RoleExaminateur  Role = "examinateur"
	RoleAdmin        Role = "admin"
)

// TypeExamen distingue un examen d'entraînement (peer-to-peer) d'un examen
// officiel encadré par un examinateur.
type TypeExamen string

const (
	ExamenEntrainement TypeExamen = "entrainement"
	ExamenOfficiel     TypeExamen = "officiel"
)

// Modalite indique la forme de l'épreuve.
type Modalite string

const (
	ModaliteQCM         Modalite = "qcm"          // écrit à choix multiple
	ModaliteCasPratique Modalite = "cas_pratique" // écrit rédactionnel
	ModaliteOral        Modalite = "oral"         // épreuve orale F2F
)

// TypeQuestion décrit la nature d'une question.
type TypeQuestion string

const (
	QuestionQCM         TypeQuestion = "qcm"
	QuestionCasPratique TypeQuestion = "cas_pratique"
)

// StatutSession suit le cycle de vie d'un passage d'examen.
type StatutSession string

const (
	SessionEnCours  StatutSession = "en_cours"
	SessionTerminee StatutSession = "terminee"
	SessionEvaluee  StatutSession = "evaluee"
)

// Utilisateur est l'entité racine. Elle porte l'authentification et le rôle.
// Relations : 1 Utilisateur -> N Examen (créés), 1 -> N Session (passées),
// 1 -> N Evaluation (données en tant que correcteur).
type Utilisateur struct {
	ID           string    `json:"id"`
	Nom          string    `json:"nom"`
	Prenom       string    `json:"prenom"`
	Email        string    `json:"email"`
	MotDePasse   string    `json:"-"` // hash bcrypt, jamais sérialisé
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// AssignationResume décrit à qui un examen est réservé (renvoyé dans la liste
// des examens pour permettre au frontend d'afficher les destinataires).
type AssignationResume struct {
	Classes   []NomID `json:"classes"`   // classes ciblées
	Etudiants []NomID `json:"etudiants"` // étudiants ciblés individuellement
}

// NomID est un couple identifiant + libellé utilisé dans AssignationResume.
type NomID struct {
	ID  string `json:"id"`
	Nom string `json:"nom"` // nom de la classe ou prénom+nom de l'étudiant
}

// SessionsResume donne la progression des sessions pour un examen ciblé :
// combien d'assignés ont terminé vs le total.
type SessionsResume struct {
	Total        int  `json:"total"`
	Terminees    int  `json:"terminees"`
	TousTermines bool `json:"tous_termines"`
}

// Examen représente un sujet d'entraînement ou une épreuve officielle.
// Relations : 1 Examen -> N Question, 1 Examen -> 0..1 GrilleCorrection,
// N Examen <- 1 Utilisateur (créateur).
type Examen struct {
	ID              string     `json:"id"`
	Titre           string     `json:"titre"`
	Description     string     `json:"description"`
	Type            TypeExamen `json:"type"`
	Modalite        Modalite   `json:"modalite"`
	DureeMin        int        `json:"duree_min"`
	Tags            []string   `json:"tags"`
	CreateurID      string     `json:"createur_id"`
	CreateurNom     string     `json:"createur_nom"`    // prénom + nom dénormalisé
	CreatedAt       time.Time  `json:"created_at"`
	DisponibleDe    *time.Time `json:"disponible_de,omitempty"`
	DisponibleJusqua *time.Time `json:"disponible_jusqu_a,omitempty"`

	// Champs d'agrégation renvoyés par l'API (non stockés tels quels).
	Questions      []Question         `json:"questions,omitempty"`
	Grille         *GrilleCorrection  `json:"grille,omitempty"`
	Assignations   *AssignationResume `json:"assignations,omitempty"` // nil = public
	SessionsResume *SessionsResume    `json:"sessions_resume,omitempty"` // staff uniquement
}

// Question appartient à un examen. Les propositions ne servent qu'aux QCM.
type Question struct {
	ID           string       `json:"id"`
	ExamenID     string       `json:"examen_id"`
	Enonce       string       `json:"enonce"`
	Type         TypeQuestion `json:"type"`
	Propositions []string     `json:"propositions,omitempty"` // options QCM
	BonneReponse *int         `json:"bonne_reponse,omitempty"` // index correct (QCM)
	Points       float64      `json:"points"`
	Ordre        int          `json:"ordre"`
}

// Session est un passage d'examen par un étudiant. Elle porte l'état du
// chronomètre et le statut. Relations : N Session <- 1 Utilisateur,
// N Session <- 1 Examen, 1 Session -> N Reponse, 1 Session -> 0..N Evaluation.
type Session struct {
	ID          string        `json:"id"`
	ExamenID    string        `json:"examen_id"`
	EtudiantID  string        `json:"etudiant_id"`
	Statut      StatutSession `json:"statut"`
	DebutAt     time.Time     `json:"debut_at"`
	FinAt       *time.Time    `json:"fin_at,omitempty"`
	TempsRestant int          `json:"temps_restant"` // secondes, pour le timer
	ScoreAuto   *float64      `json:"score_auto,omitempty"` // note QCM auto-calculée

	Reponses []Reponse `json:"reponses,omitempty"`
}

// Reponse est la réponse d'un étudiant à une question dans une session.
type Reponse struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	QuestionID string    `json:"question_id"`
	Choix      *int      `json:"choix,omitempty"`  // index choisi (QCM)
	Texte      string    `json:"texte,omitempty"`  // réponse rédigée (cas pratique)
	Correct    *bool     `json:"correct,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// GrilleCorrection définit le barème d'un examen. Relations :
// 1 GrilleCorrection <- 1 Examen, 1 GrilleCorrection -> N Critere.
type GrilleCorrection struct {
	ID       string    `json:"id"`
	ExamenID string    `json:"examen_id"`
	Titre    string    `json:"titre"`
	Criteres []Critere `json:"criteres"`
}

// Critere est une ligne de barème avec un poids en points.
type Critere struct {
	ID       string  `json:"id"`
	GrilleID string  `json:"grille_id"`
	Libelle  string  `json:"libelle"`
	PointsMax float64 `json:"points_max"`
	Ordre    int     `json:"ordre"`
}

// Evaluation est une correction d'une session, réalisée soit par un pair
// (peer-to-peer), soit par un examinateur. Relations :
// N Evaluation <- 1 Session, N Evaluation <- 1 Utilisateur (correcteur).
type Evaluation struct {
	ID           string        `json:"id"`
	SessionID    string        `json:"session_id"`
	CorrecteurID string        `json:"correcteur_id"`
	Remarques    string        `json:"remarques"`
	NoteTotale   float64       `json:"note_totale"`
	NoteVisible  bool          `json:"note_visible"` // si false, cachée à l'étudiant
	Notes        []NoteCritere `json:"notes"`
	CreatedAt    time.Time     `json:"created_at"`
}

// NoteCritere associe des points à un critère de la grille pour une évaluation.
type NoteCritere struct {
	ID           string  `json:"id"`
	EvaluationID string  `json:"evaluation_id"`
	CritereID    string  `json:"critere_id"`
	Points       float64 `json:"points"`
}
