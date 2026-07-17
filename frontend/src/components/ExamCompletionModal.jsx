import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { useNavigate } from 'react-router-dom'
import './ExamCompletionModal.css'

export default function ExamCompletionModal({ examen, onClose }) {
  const navigate = useNavigate()
  const [sessions, setSessions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    // On va récupérer les sessions de cet examen pour le staff.
    // On peut utiliser une route API existante ou filtrer. 
    // Commençons par appeler /api/sessions/active-orals ou faire une requête sur les sessions d'un examen.
    // Oh, attendons, pour lister les sessions d'un examen :
    // On a besoin d'obtenir toutes les sessions liées à cet examen. Est-ce qu'on a un endpoint pour ça ?
    // Voyons s'il y a un endpoint pour lister les sessions d'un examen. Regardons internal/handlers/sessions.go
    // Il n'y a pas de endpoint direct listant toutes les sessions d'un examen pour le staff,
    // mais on a /api/sessions/to-evaluate (ListSessionsToEvaluate) ou on peut ajouter un petit handler ou appeler un endpoint.
    // Lisons ListSessionsToEvaluate :
    // "SELECT s.id, s.examen_id, s.statut, e.titre, u.prenom, u.nom FROM sessions s JOIN examens e ON e.id = s.examen_id JOIN utilisateurs u ON u.id = s.etudiant_id"
    // C'est parfait, on peut appeler ListSessionsToEvaluate pour filtrer sur examen_id, ou alors on peut faire une route spécifique.
    // Faisons au plus simple et robuste: filtrer sur ListSessionsToEvaluate ou modifier ListSessionsToEvaluate pour le personnel ou créer un petit endpoint.
    // Attends, on peut faire un fetch vers /api/evaluations/given ou /api/sessions/to-evaluate.
    // Créons plutôt un handler simple ou réutilisons /api/sessions/to-evaluate.
    // Filtrons /api/sessions/to-evaluate. Elle retourne les sessions d'autres utilisateurs. Comme l'examinateur n'est pas le candidat, 
    // /api/sessions/to-evaluate retournera toutes les sessions d'étudiants de cet examen !
    // Vérifions dans le handler: `ListSessionsToEvaluate` filtre `s.etudiant_id <> $1`. Comme l'examinateur a un ID de staff, 
    // cela retournera toutes les sessions d'étudiants.
    api('/sessions/to-evaluate')
      .then((data) => {
        setSessions(data.filter((s) => s.examen_id === examen.id))
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [examen.id])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-container" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2><i className="bi bi-bullseye" /> Destinataires de : {examen.titre}</h2>
          <button className="btn-close" onClick={onClose}><i className="bi bi-x-lg" /></button>
        </div>

        <div className="modal-body">
          <p className="muted">
            Tous les étudiants ciblés ont terminé cet examen. Veuillez cliquer sur une session ci-dessous pour l&apos;évaluer ou revoir la copie.
          </p>

          {loading ? (
            <div className="center" style={{ padding: 30 }}><div className="spinner" /></div>
          ) : error ? (
            <div className="alert alert-error">{error}</div>
          ) : sessions.length === 0 ? (
            <p className="muted">Aucune session trouvée pour cet examen.</p>
          ) : (
            <div className="completion-sessions-list">
              {sessions.map((s) => (
                <div key={s.id} className="completion-session-item">
                  <div className="session-item-info">
                    <strong>{s.etudiant}</strong>
                    <span className={`tag tag-statut-${s.statut}`}>{s.statut === 'evaluee' ? 'Évaluée' : 'Terminée'}</span>
                  </div>
                  <button
                    className="btn btn-primary btn-small"
                    onClick={() => {
                      navigate(`/sessions/${s.id}`)
                      onClose()
                    }}
                  >
                    Voir la copie
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose}>Fermer</button>
        </div>
      </div>
    </div>
  )
}
