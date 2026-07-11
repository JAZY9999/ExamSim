// Statistiques — le contenu s'adapte au rôle :
//  - Étudiant : historique de ses sessions (score auto QCM ou note d'évaluation)
//    + sessions de camarades à évaluer (peer-to-peer).
//  - Examinateur/Admin : historique des évaluations réalisées
//    (« X est passé et a obtenu n/20 »).
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import './PageCommon.css'

export default function StatsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const isExaminateur = user?.role === 'examinateur' || user?.role === 'admin'

  const [sessions, setSessions] = useState([])
  const [toEval, setToEval] = useState([])
  const [given, setGiven] = useState([])
  const [error, setError] = useState('')

  useEffect(() => {
    if (isExaminateur) {
      api('/evaluations/given').then(setGiven).catch((e) => setError(e.message))
    } else {
      api('/sessions/mine').then(setSessions).catch((e) => setError(e.message))
      api('/sessions/to-evaluate').then(setToEval).catch(() => {})
    }
  }, [isExaminateur])

  const fmt = (d) => new Date(d).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })

  // Note affichée : score auto (QCM) ou note d'évaluation (oral / peer-to-peer).
  function noteCell(s) {
    if (s.score_auto != null) return `${s.score_auto.toFixed(1)} / 20`
    if (s.note_eval != null) return `${s.note_eval.toFixed(1)} / ${s.note_max ?? 20}`
    return '—'
  }

  return (
    <div>
      <div className="page-hero">
        <h1>Statistiques</h1>
        <p className="muted">
          {isExaminateur
            ? 'Historique des épreuves que vous avez évaluées.'
            : 'Suivez votre progression et évaluez le travail de vos camarades.'}
        </p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {isExaminateur ? (
        <>
          <h2 className="section-title">Évaluations réalisées</h2>
          <table className="table">
            <thead>
              <tr><th>Candidat</th><th>Épreuve</th><th>Date</th><th>Note</th><th>Remarques</th></tr>
            </thead>
            <tbody>
              {given.map((ev) => (
                <tr key={ev.id} className="row-link" title="Voir la copie détaillée"
                  onClick={() => navigate(`/sessions/${ev.session_id}`)}>
                  <td><strong>{ev.etudiant}</strong></td>
                  <td>{ev.examen_titre}</td>
                  <td>{fmt(ev.created_at)}</td>
                  <td>
                    <span className="tag tag-success">
                      {ev.note_totale.toFixed(1)} / {ev.note_max ?? 20}
                    </span>
                  </td>
                  <td className="muted">{ev.remarques || '—'}</td>
                </tr>
              ))}
              {given.length === 0 && (
                <tr><td colSpan={5} className="muted">
                  Aucune évaluation pour l'instant. Rejoignez un oral en cours
                  depuis l'accueil pour noter un candidat.
                </td></tr>
              )}
            </tbody>
          </table>
        </>
      ) : (
        <>
          <h2 className="section-title">Mes sessions</h2>
          <table className="table" style={{ marginBottom: 40 }}>
            <thead>
              <tr><th>Examen</th><th>Date</th><th>Statut</th><th>Note</th></tr>
            </thead>
            <tbody>
              {sessions.map((s) => (
                <tr key={s.id} className="row-link" title="Voir ma copie détaillée"
                  onClick={() => navigate(`/sessions/${s.id}`)}>
                  <td>{s.examen_titre}</td>
                  <td>{fmt(s.debut_at)}</td>
                  <td><StatutBadge statut={s.statut} /></td>
                  <td>{noteCell(s)}</td>
                </tr>
              ))}
              {sessions.length === 0 && <tr><td colSpan={4} className="muted">Aucune session pour l'instant.</td></tr>}
            </tbody>
          </table>

          <h2 className="section-title">À évaluer (peer-to-peer)</h2>
          <table className="table">
            <thead>
              <tr><th>Étudiant</th><th>Examen</th><th>Statut</th></tr>
            </thead>
            <tbody>
              {toEval.map((s) => (
                <tr key={s.id} className="row-link" title="Consulter et évaluer cette copie"
                  onClick={() => navigate(`/sessions/${s.id}`)}>
                  <td>{s.etudiant}</td>
                  <td>{s.examen_titre}</td>
                  <td><StatutBadge statut={s.statut} /></td>
                </tr>
              ))}
              {toEval.length === 0 && <tr><td colSpan={3} className="muted">Rien à évaluer pour le moment.</td></tr>}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}

function StatutBadge({ statut }) {
  const map = {
    en_cours: ['tag-warning', 'En cours'],
    terminee: ['tag', 'Terminée'],
    evaluee:  ['tag-success', 'Évaluée'],
  }
  const [cls, label] = map[statut] || ['tag', statut]
  return <span className={`tag ${cls}`}>{label}</span>
}
