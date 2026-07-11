-- Schéma relationnel de la plateforme de simulation d'examens.
-- Ce fichier est exécuté au démarrage du backend (idempotent grâce à IF NOT EXISTS).
-- Il correspond au diagramme de classes UML du dossier de conception.

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- pour gen_random_uuid()

-- === Utilisateur : acteur racine (étudiant / examinateur / admin) =========
CREATE TABLE IF NOT EXISTS utilisateurs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nom         TEXT NOT NULL,
    prenom      TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    mot_de_passe TEXT NOT NULL,           -- hash bcrypt
    role        TEXT NOT NULL CHECK (role IN ('etudiant','examinateur','admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- === Examen : sujet d'entraînement ou épreuve officielle ==================
CREATE TABLE IF NOT EXISTS examens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titre       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL CHECK (type IN ('entrainement','officiel')),
    modalite    TEXT NOT NULL CHECK (modalite IN ('qcm','cas_pratique','oral')),
    duree_min   INTEGER NOT NULL DEFAULT 30,
    tags        TEXT[] NOT NULL DEFAULT '{}',
    createur_id UUID NOT NULL REFERENCES utilisateurs(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- === Question : rattachée à un examen (QCM ou cas pratique) ===============
CREATE TABLE IF NOT EXISTS questions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examen_id     UUID NOT NULL REFERENCES examens(id) ON DELETE CASCADE,
    enonce        TEXT NOT NULL,
    type          TEXT NOT NULL CHECK (type IN ('qcm','cas_pratique')),
    propositions  TEXT[] NOT NULL DEFAULT '{}',
    bonne_reponse INTEGER,                 -- index de la bonne proposition (QCM)
    points        DOUBLE PRECISION NOT NULL DEFAULT 1,
    ordre         INTEGER NOT NULL DEFAULT 0
);

-- === GrilleCorrection : barème d'un examen ================================
CREATE TABLE IF NOT EXISTS grilles (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examen_id  UUID NOT NULL REFERENCES examens(id) ON DELETE CASCADE,
    titre      TEXT NOT NULL DEFAULT 'Grille de correction'
);

CREATE TABLE IF NOT EXISTS criteres (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grille_id  UUID NOT NULL REFERENCES grilles(id) ON DELETE CASCADE,
    libelle    TEXT NOT NULL,
    points_max DOUBLE PRECISION NOT NULL DEFAULT 5,
    ordre      INTEGER NOT NULL DEFAULT 0
);

-- === Session : passage d'un examen par un étudiant ========================
CREATE TABLE IF NOT EXISTS sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examen_id     UUID NOT NULL REFERENCES examens(id) ON DELETE CASCADE,
    etudiant_id   UUID NOT NULL REFERENCES utilisateurs(id) ON DELETE CASCADE,
    statut        TEXT NOT NULL DEFAULT 'en_cours' CHECK (statut IN ('en_cours','terminee','evaluee')),
    debut_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    fin_at        TIMESTAMPTZ,
    temps_restant INTEGER NOT NULL DEFAULT 0,
    score_auto    DOUBLE PRECISION
);

-- === Reponse : réponse d'un étudiant à une question dans une session ======
CREATE TABLE IF NOT EXISTS reponses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    choix       INTEGER,                   -- index choisi (QCM)
    texte       TEXT NOT NULL DEFAULT '',  -- réponse rédigée (cas pratique)
    correct     BOOLEAN,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, question_id)
);

-- === Evaluation : correction d'une session (pair ou examinateur) ==========
CREATE TABLE IF NOT EXISTS evaluations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    correcteur_id UUID NOT NULL REFERENCES utilisateurs(id) ON DELETE CASCADE,
    remarques     TEXT NOT NULL DEFAULT '',
    note_totale   DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notes_critere (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evaluation_id UUID NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    critere_id    UUID NOT NULL REFERENCES criteres(id) ON DELETE CASCADE,
    points        DOUBLE PRECISION NOT NULL DEFAULT 0
);

-- === Classes : regroupement des étudiants (ex: « B2 Informatique ») =======
CREATE TABLE IF NOT EXISTS classes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nom        TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS classe_membres (
    classe_id      UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    utilisateur_id UUID NOT NULL REFERENCES utilisateurs(id) ON DELETE CASCADE,
    PRIMARY KEY (classe_id, utilisateur_id)
);

-- === Assignations : à qui un examen est destiné ============================
-- Un examen SANS assignation est visible par tous les étudiants.
-- Sinon, il n'est visible que par les classes et/ou étudiants ciblés.
CREATE TABLE IF NOT EXISTS examen_assignations (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examen_id      UUID NOT NULL REFERENCES examens(id) ON DELETE CASCADE,
    classe_id      UUID REFERENCES classes(id) ON DELETE CASCADE,
    utilisateur_id UUID REFERENCES utilisateurs(id) ON DELETE CASCADE,
    CHECK (classe_id IS NOT NULL OR utilisateur_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_assign_examen ON examen_assignations(examen_id);

-- === Journal d'audit : traçabilité de toutes les actions sensibles ========
-- acteur_id est en ON DELETE SET NULL : la trace SURVIT à la suppression du
-- compte (exigence d'audit — on ne peut pas effacer son historique en
-- supprimant un utilisateur).
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acteur_id   UUID REFERENCES utilisateurs(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,
    cible_type  TEXT NOT NULL DEFAULT '',
    cible_id    TEXT NOT NULL DEFAULT '',
    details     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index utiles aux requêtes fréquentes.
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_questions_examen ON questions(examen_id);
CREATE INDEX IF NOT EXISTS idx_sessions_etudiant ON sessions(etudiant_id);
CREATE INDEX IF NOT EXISTS idx_reponses_session ON reponses(session_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_session ON evaluations(session_id);

-- === Évolutions (idempotentes sur BDD existante) ===========================

-- Fenêtre de disponibilité d'un examen : optionnelle (NULL = toujours visible).
-- L'étudiant ne voit l'examen que si now() est dans la fenêtre.
ALTER TABLE examens ADD COLUMN IF NOT EXISTS disponible_de      TIMESTAMPTZ;
ALTER TABLE examens ADD COLUMN IF NOT EXISTS disponible_jusqu_a TIMESTAMPTZ;

-- Contrôle de visibilité de la note : l'examinateur choisit si l'étudiant
-- peut voir sa note. Par défaut false (non publiée).
ALTER TABLE evaluations ADD COLUMN IF NOT EXISTS note_visible BOOLEAN NOT NULL DEFAULT false;

