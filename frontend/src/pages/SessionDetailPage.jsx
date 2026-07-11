// Détail d'une session (« la copie ») — page de traçabilité centrale :
//  - métadonnées : candidat, épreuve, dates, statut, score ;
//  - les réponses question par question (choix, bonne réponse, ✓/✗, texte) ;
//  - chaque évaluation avec son détail PAR CRITÈRE, le correcteur et la date ;
//  - si le lecteur y est autorisé (pair ou examinateur), le formulaire
//    d'évaluation est intégré ici (grille de critères, ou note directe si
//    l'examen n'a pas de grille) — c'est la page du peer-to-peer.
//  - Ajout des fonctions de modification d'évaluation, message pré-enregistrés, 
//    et de gestion de la visibilité de la note par l'examinateur.
import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import './PageCommon.css'
import './SessionDetailPage.css'

const PRESETS = [
  "Excellent travail ! Les objectifs sont atteints avec brio.",
  "Très bon travail, les concepts sont bien maîtrisés.",
  "Travail satisfaisant. Des points d'amélioration subsistent.",
  "Bonne compréhension globale mais manque de rigueur ou de précision.",
  "Résultat correct, mais certains aspects clés doivent être approfondis.",
  "Des efforts visibles mais des lacunes importantes subsistent.",
  "Travail insuffisant. Les notions fondamentales ne sont pas acquises.",
  "Rendu incomplet, plusieurs questions importantes n'ont pas été traitées.",
  "Hors sujet sur une partie substantielle du travail. Ré-étudiez le cours.",
  "Plagiat ou travail identique détecté. Une explication est requise."
]

export default function SessionDetailPage() {
  const { id: sessionId } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()

  const [data, setData] = useState(null)
  const [error, setError] = useState('')

  // Formulaire d'évaluation
  const [notes, setNotes] = useState({})             // critereId -> points
  const [noteDirecte, setNoteDirecte] = useState('') // si pas de grille
  const [remarques, setRemarques] = useState('')
  const [noteVisible, setNoteVisible] = useState(false)
  const [busy, setBusy] = useState(false)

  // Mode édition d'évaluation existante
  const [editingEvalId, setEditingEvalId] = useState(null)

  function load() {
    api(`/sessions/${sessionId}/detail`)
      .then(setData)
      .catch((e) => setError(e.message))
  }
  useEffect(load, [sessionId])

  if (error) return <div className="alert alert-error">{error}</div>
  if (!data) return <div className="center" style={{ padding: 60 }}><div className="spinner" /></div>

  const { session, examen, etudiant, reponses, evaluations, peut_evaluer, is_staff } = data
  const repByQuestion = Object.fromEntries((reponses || []).map((r) => [r.question_id, r]))
  const grille = examen.grille
  const fmt = (d) => new Date(d).toLocaleString('fr-FR', { dateStyle: 'medium', timeStyle: 'short' })

  // Remplir le formulaire avec une évaluation existante pour modification
  function démarrerEdition(ev) {
    setEditingEvalId(ev.id)
    setRemarques(ev.remarques || '')
    setNoteVisible(ev.note_visible || false)
    if (grille) {
      const notesMap = {}
      ev.notes.forEach((n) => {
        notesMap[n.critere_id] = n.points
      })
      setNotes(notesMap)
    } else {
      setNoteDirecte(String(ev.note_totale || 0))
    }
    // Scroll jusqu'au formulaire
    document.querySelector('.eval-form')?.scrollIntoView({ behavior: 'smooth' })
  }

  function annulerEdition() {
    setEditingEvalId(null)
    setRemarques('')
    setNotes({})
    setNoteDirecte('')
    setNoteVisible(false)
  }

  async function toggleVisibilité(evId, currentVisible) {
    try {
      await api(`/api/evaluations/${evId}/visibilite`, {
        method: 'PATCH',
        body: { visible: !currentVisible }
      })
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  async function envoyerEvaluation(e) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const body = {
        remarques,
        note_visible: noteVisible
      }
      if (grille) {
        body.notes = grille.criteres.map((c) => ({
          critere_id: c.id,
          points: Number(notes[c.id] || 0),
        }))
      } else {
        body.note_totale = Number(noteDirecte) || 0
      }

      if (editingEvalId) {
        // Mode modification
        await api(`/evaluations/${editingEvalId}`, { method: 'PATCH', body })
      } else {
        // Mode création
        await api(`/sessions/${sessionId}/evaluations`, { method: 'POST', body })
      }

      annulerEdition()
      load()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="session-detail">
      <button className="btn btn-ghost" onClick={() => navigate(-1)}>← Retour</button>

      {/* --- En-tête : traçabilité de la session --- */}
      <div className="card detail-header">
        <div>
          <h1>{examen.titre}</h1>
          <p className="muted">{examen.description}</p>
          <div className="detail-meta">
            <span className="tag">{etudiant.prenom} {etudiant.nom}</span>
            <span className="muted">{etudiant.email}</span>
            <StatutBadge statut={session.statut} />
          </div>
        </div>
        <div className="detail-dates">
          <div><span className="muted">Début :</span> {fmt(session.debut_at)}</div>
          {session.fin_at && <div><span className="muted">Fin :</span> {fmt(session.fin_at)}</div>}
          {session.score_auto != null && (
            <div className="detail-score">Score auto : <strong>{session.score_auto.toFixed(1)} / 20</strong></div>
          )}
        </div>
      </div>

      {/* --- La copie : réponses question par question --- */}
      {(examen.questions || []).length > 0 && (
        <>
          <h2 className="section-title">Copie — {examen.questions.length} question(s)</h2>
          {examen.questions.map((q, i) => {
            const rep = repByQuestion[q.id]
            return (
              <div key={q.id} className="card question-detail">
                <div className="question-detail-head">
                  <strong>Question {i + 1}</strong>
                  <span className="muted">{q.points} pt(s)</span>
                  {q.type === 'qcm' && rep && (
                    rep.correct
                      ? <span className="tag tag-success">✓ Correct</span>
                      : <span className="tag tag-danger">✗ Incorrect</span>
                  )}
                  {!rep && <span className="tag tag-warning">Sans réponse</span>}
                </div>
                <p className="question-detail-enonce">{q.enonce}</p>

                {q.type === 'qcm' ? (
                  <ul className="props-detail">
                    {q.propositions.map((p, j) => {
                      const isChoix = rep?.choix === j
                      const isBonne = q.bonne_reponse === j
                      return (
                        <li key={j} className={
                          'prop-detail' +
                          (isBonne ? ' bonne' : '') +
                          (isChoix && !isBonne ? ' mauvaise' : '')
                        }>
                          {p}
                          {isBonne && <span className="prop-flag">bonne réponse</span>}
                          {isChoix && <span className="prop-flag choix">choix du candidat</span>}
                        </li>
                      )
                    })}
                  </ul>
                ) : (
                  <div className="texte-reponse">
                    {rep?.texte
                      ? <p>{rep.texte}</p>
                      : <p className="muted">Aucune réponse rédigée.</p>}
                  </div>
                )}
              </div>
            )
          })}
        </>
      )}

      {/* --- Section évaluations --- */}
      <h2 className="section-title">Évaluations ({evaluations?.length || 0})</h2>
      {(!evaluations || evaluations.length === 0) ? (
        <div className="alert alert-info">
          La correction n&apos;est pas encore disponible pour cette session.
        </div>
      ) : (
        evaluations.map((ev) => {
          // L'examinateur ou le correcteur original peut modifier l'évaluation
          const peutModifier = is_staff || ev.correcteur_id === user?.id
          return (
            <div key={ev.id} className="card eval-detail">
              <div className="eval-detail-head">
                <div>
                  <strong>{ev.correcteur}</strong>
                  <span className="muted"> · {fmt(ev.created_at)}</span>
                  {is_staff && (
                    <span className={`tag ${ev.note_visible ? 'tag-success' : 'tag-warning'}`} style={{ marginLeft: 8 }}>
                      {ev.note_visible ? '👁 Publiée' : '🙈 Cachée'}
                    </span>
                  )}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <span className="tag tag-success" style={{ fontSize: 15, fontWeight: 700 }}>
                    {ev.note_totale.toFixed(1)} pts
                  </span>
                  {peutModifier && (
                    <button className="btn btn-ghost btn-small" onClick={() => démarrerEdition(ev)} title="Modifier l'évaluation">
                      ✏️ Modifier
                    </button>
                  )}
                  {is_staff && (
                    <button
                      className={`btn btn-small ${ev.note_visible ? 'btn-ghost' : 'btn-primary'}`}
                      onClick={() => toggleVisibilité(ev.id, ev.note_visible)}
                    >
                      {ev.note_visible ? 'Cacher' : 'Publier'}
                    </button>
                  )}
                </div>
              </div>
              {ev.notes?.length > 0 && (
                <table className="table criteres-table">
                  <tbody>
                    {ev.notes.map((n) => (
                      <tr key={n.critere_id}>
                        <td>{n.libelle}</td>
                        <td className="critere-note">{n.points} / {n.points_max}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              {ev.remarques && <p className="eval-remarques-detail">« {ev.remarques} »</p>}
            </div>
          )
        })
      )}

      {/* --- Formulaire d'évaluation (peer-to-peer / examinateur) --- */}
      {peut_evaluer && (
        <form className="card eval-form" onSubmit={envoyerEvaluation}>
          <h2 className="section-title">
            {editingEvalId ? '✏️ Modifier l\'évaluation' : 'Évaluer cette copie'}
          </h2>

          {grille ? (
            grille.criteres.map((c) => (
              <div key={c.id} className="critere-row" style={{ marginBottom: 14 }}>
                <div className="critere-form-head">
                  <span>{c.libelle}</span>
                  <span>{notes[c.id] || 0} / {c.points_max}</span>
                </div>
                <input type="range" min="0" max={c.points_max} step="0.5"
                  className="slider-detail"
                  value={notes[c.id] || 0}
                  onChange={(e) => setNotes({ ...notes, [c.id]: e.target.value })} />
              </div>
            ))
          ) : (
            <div className="field">
              <label>Note attribuée (sur 20)</label>
              <input className="input" type="number" min="0" max="20" step="0.5"
                value={noteDirecte} onChange={(e) => setNoteDirecte(e.target.value)} required />
            </div>
          )}

          {/* Messages pré-enregistrés */}
          {is_staff && (
            <div className="field">
              <label>Commentaires pré-enregistrés</label>
              <select
                className="select"
                onChange={(e) => {
                  if (e.target.value) {
                    setRemarques(e.target.value)
                    e.target.value = "" // reset select
                  }
                }}
                defaultValue=""
              >
                <option value="" disabled>Sélectionner un commentaire type...</option>
                {PRESETS.map((p, idx) => (
                  <option key={idx} value={p}>{p}</option>
                ))}
              </select>
            </div>
          )}

          <div className="field">
            <label>Remarques</label>
            <textarea className="textarea" value={remarques}
              onChange={(e) => setRemarques(e.target.value)}
              placeholder="Points forts, axes d'amélioration..." />
          </div>

          {/* Option de publication de la note (seulement pour le staff) */}
          {is_staff && (
            <div className="field checkbox-field" style={{ marginBottom: 20 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={noteVisible}
                  onChange={(e) => setNoteVisible(e.target.checked)}
                  style={{ width: 18, height: 18 }}
                />
                <strong>Rendre la note et les remarques visibles à l&apos;étudiant</strong>
              </label>
            </div>
          )}

          <div style={{ display: 'flex', gap: 10 }}>
            <button className="btn btn-primary" disabled={busy}>
              {busy ? '...' : (editingEvalId ? 'Modifier l\'évaluation' : 'Enregistrer l\'évaluation')}
            </button>
            {editingEvalId && (
              <button type="button" className="btn btn-ghost" onClick={annulerEdition}>
                Annuler
              </button>
            )}
          </div>
        </form>
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
