# ExamSim — Plateforme de Simulation d'Examens

Application web d'entraînement pour étudiants : génération de sujets,
chronométrage des épreuves écrites (QCM / cas pratiques), passage d'oraux en
**face-à-face** avec **chronomètre synchronisé en temps réel** et **grille de
correction** partagée (peer-to-peer ou examinateur).

Projet de fin de module **Modélisation UML 2** (ESGI). La partie développement
implémente le cahier des charges du dossier de conception.

---

## 🧱 Stack technique

| Couche | Technologie | Pourquoi |
|--------|-------------|----------|
| Frontend | **React 18 + Vite** | Interfaces réactives, idéal pour les timers temps réel |
| Backend | **Go (Golang)** + chi | Haute performance, gestion de nombreuses requêtes simultanées |
| Base de données | **PostgreSQL 16** | Base relationnelle robuste (schéma issu du diagramme de classes) |
| Temps réel | **WebSocket** (gorilla/websocket) | Timer oral synchronisé examinateur ↔ étudiant |
| Prestataire extérieur | **Jitsi** (visioconférence) | Flux vidéo de l'oral F2F, sans clé API |
| Sécurité | **JWT** + **bcrypt** | Routes protégées, mots de passe hachés |
| DevOps | **Docker Compose** | Lancement en une commande |

---

## 🚀 Démarrage rapide

### Option A — Docker (recommandé)

```bash
cd Code
docker compose up --build
```

Puis ouvrez **http://localhost:5173**.

### Option B — En local (dev)

**1. PostgreSQL** (via Docker, seulement la base) :
```bash
docker run --name examsim-db -e POSTGRES_USER=examsim -e POSTGRES_PASSWORD=examsim \
  -e POSTGRES_DB=examsim -p 5432:5432 -d postgres:16-alpine
```

**2. Backend Go** :
```bash
cd Code/backend
go run ./cmd/server
# → http://localhost:8080
```

**3. Frontend React** :
```bash
cd Code/frontend
npm install
npm run dev
# → http://localhost:5173  (proxy /api et /ws vers :8080)
```

---

## 👤 Comptes de démonstration

Créés automatiquement au premier lancement (mot de passe : `password`) :

| Email | Rôle | Usage |
|-------|------|-------|
| `etudiant@examsim.fr` | Étudiant | Passer des examens, peer-to-peer |
| `camarade@examsim.fr` | Étudiant | Second étudiant (évaluation entre pairs) |
| `prof@examsim.fr` | Examinateur | Noter les oraux en direct |
| `admin@examsim.fr` | Administrateur | Gérer les comptes |

---

## 🎬 Scénario de démonstration (soutenance)

1. **Administration (gestion des classes)** :
   - Se connecter en `admin@examsim.fr` -> onglet **Administration** -> **Gérer les classes** (ou `http://localhost:5173/admin/classes`).
   - Créer une classe (ex: *Master 2 Cyber*), y ajouter/retirer des étudiants.
2. **Création & ciblage d'un examen (Professeur)** :
   - Se connecter en `prof@examsim.fr`.
   - Aller sur **Créer un sujet** (ou `http://localhost:5173/creer`).
   - Configurer un examen (modalité QCM, Cas Pratique ou Oral).
   - Définir une **fenêtre de disponibilité** (Début / Fin).
   - À l'étape **Destinataires**, sélectionner une classe (ex: *Master 2 Cyber*) ou quelques étudiants précis.
3. **Passage d'épreuve (Étudiant)** :
   - Se connecter en tant qu'étudiant ciblé (`etudiant@examsim.fr`).
   - L'épreuve s'affiche sur l'accueil (avec le badge du professeur créateur).
   - Démarrer, répondre et soumettre.
   - De retour sur l'accueil, l'examen a **disparu** de la liste pour ne plus lui être reproposé.
4. **Correction en direct & publication des notes (Professeur)** :
   - Se connecter en `prof@examsim.fr`.
   - Sur l'accueil, la carte de l'examen montre une bordure verte et affiche le badge cliquable `✅ Tous terminés` (puisque tous les assignés l'ont passé).
   - Cliquer sur le badge pour ouvrir le pop-up, puis sur **Voir la copie**.
   - Évaluer la copie : utiliser le menu déroulant des **commentaires pré-enregistrés**, choisir de publier ou cacher la note à l'étudiant via la case à cocher, et enregistrer. Le prof peut à tout moment modifier sa correction en cliquant sur *Modifier*.
5. **Tableau de bord de classe style Socrative** :
   - Aller sur **Mes sujets** (`http://localhost:5173/mes-sujets`) -> cliquer sur **📊 Résultats** à côté de l'examen.
   - Analyser le tableau de bord de classe en utilisant les boutons interactifs (*Afficher les noms*, *Afficher les réponses*, *Afficher les couleurs*) et inspecter les moyennes par colonne.

---

## 🗺️ Architecture

```
Navigateur (React SPA)
   │  REST /api/*            │  WebSocket /ws/oral/{session}
   ▼                        ▼
        Backend Go (chi router)
   │  auth JWT · handlers · Hub timer
   ▼
   PostgreSQL 16  ◄── entités métier

Prestataire extérieur : Jitsi (iframe visio) ── appelé depuis le navigateur
```

---

## 📁 Structure du code

```
Code/
├── backend/                  # API Go
│   ├── cmd/server/main.go    # point d'entrée + routing
│   └── internal/
│       ├── models/           # Entités métier enrichies (grille, classes, assignations)
│       ├── db/               # schema.sql + connexion + seed (dont seed de classes de démo)
│       ├── middleware/       # JWT + contrôle de rôle
│       ├── handlers/         # auth, examens, sessions, évaluations, classes, admin
│       └── ws/               # Hub du timer oral synchronisé
├── frontend/                 # SPA React
│   └── src/
│       ├── pages/            # AuthPage, Home, ExamRun, Oral, Stats, Admin, AdminClasses, MyExams, ExamResults
│       ├── components/       # DashboardLayout, ExamCard, ExamCompletionModal
│       ├── context/          # AuthContext
│       └── api/              # client HTTP
└── docker-compose.yml
```

---

## 🔌 Endpoints principaux

| Méthode | Route | Rôle |
|---------|-------|------|
| POST | `/api/auth/register` | Inscription |
| POST | `/api/auth/login` | Connexion (→ JWT) |
| GET  | `/api/examens` | Liste des examens (cartes, filtrage par dispo/passage) |
| GET  | `/api/examens/{id}` | Détail (questions + grille) |
| POST | `/api/examens` | Créer un sujet (avec ciblage & dispo de/à) |
| PATCH| `/api/examens/{id}` | Modifier un sujet (créateur / admin) |
| DELETE| `/api/examens/{id}` | Supprimer un sujet (créateur / admin) |
| GET  | `/api/examens/{id}/resultats` | Matrice des notes et réponses (style Socrative) |
| POST | `/api/sessions` | Démarrer un passage |
| POST | `/api/sessions/{id}/submit` | Soumettre (correction auto QCM) |
| GET  | `/api/sessions/mine` | Historique / statistiques |
| GET  | `/api/sessions/to-evaluate` | Sessions à évaluer (peer-to-peer / staff) |
| POST | `/api/sessions/{id}/evaluations` | Enregistrer une évaluation |
| PATCH| `/api/evaluations/{id}` | Modifier une évaluation (correcteur original / admin) |
| PATCH| `/api/evaluations/{id}/visibilite` | Publier ou masquer la note à l'étudiant |
| GET  | `/api/classes` | Liste des classes de cours |
| POST | `/api/classes` | Créer une classe (admin) |
| GET  | `/api/admin/utilisateurs` | Liste des comptes (admin) |
| WS   | `/ws/oral/{sessionID}` | Timer oral synchronisé |


---

## 🔐 Sécurité

- Mots de passe **hachés avec bcrypt** (jamais stockés en clair).
- Routes API **protégées par JWT** (`Authorization: Bearer <token>`).
- Contrôle d'accès **par rôle** (middleware `RequireRole`).
- Vérification d'appartenance des sessions (un étudiant ne soumet que sa copie).
