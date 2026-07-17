// Page « Mes sujets » — liste tous les examens créés par l'examinateur connecté.
// Permet de modifier ou supprimer chaque examen directement depuis le tableau.
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import './PageCommon.css'
import './MyExamsPage.css'

const MODALITE = { qcm: 'QCM', cas_pratique: 'Cas pratique', oral: 'Oral' }
const TYPE = { officiel: 'Officiel', entrainement: 'Entraînement' }

function fmtDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('fr-FR', { day: '2-digit', month: 'short', year: 'numeric' })
}
function fmtDateTime(iso) {
  if (!iso) return null
  return new Date(iso).toLocaleString('fr-FR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

export default function MyExamsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [examens, setExamens] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [deleting, setDeleting] = useState(null) // id en cours de suppression

  function load() {
    api('/examens')
      .then((all) => setExamens(all.filter((e) => e.createur_id === user?.id)))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }
  useEffect(load, [user])

  async function supprimer(e) {
    if (!window.confirm(`Supprimer définitivement « ${e.titre} » ?`)) return
    setDeleting(e.id)
    try {
      await api(`/examens/${e.id}`, { method: 'DELETE' })
      load()
    } catch (err) {
      setError(err.message)
    } finally {
      setDeleting(null)
    }
  }

  return (
    <div>
      <div className="page-hero" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1>Mes sujets</h1>
          <p className="muted">Retrouvez tous vos examens et entraînements créés — modifiez ou supprimez-les ici.</p>
        </div>
        <button className="btn btn-primary" onClick={() => navigate('/creer')}><i className="bi bi-plus-lg" /> Nouveau sujet</button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="center" style={{ padding: 60 }}><div className="spinner" /></div>
      ) : examens.length === 0 ? (
        <div className="my-exams-empty">
          <p>Vous n&apos;avez encore créé aucun sujet.</p>
          <button className="btn btn-primary" onClick={() => navigate('/creer')}><i className="bi bi-plus-lg" /> Créer mon premier sujet</button>
        </div>
      ) : (
        <div className="my-exams-list">
          {examens.map((e) => {
            const resume = e.sessions_resume
            return (
              <div key={e.id} className={`my-exam-card${resume?.tous_termines ? ' my-exam-card--done' : ''}`}>
                {/* Colonne gauche : info principale */}
                <div className="my-exam-info">
                  <div className="my-exam-top">
                    <span className={`mod-pill mod-${e.modalite}`}>{MODALITE[e.modalite]}</span>
                    <span className="muted" style={{ fontSize: 13 }}>{TYPE[e.type]}</span>
                    {e.assignations && (
                      <span className="tag" style={{ fontSize: 12 }}>
                        {(e.assignations.classes?.length || 0) + (e.assignations.etudiants?.length || 0)} destinataire(s)
                      </span>
                    )}
                  </div>
                  <h3 className="my-exam-title">{e.titre}</h3>
                  {e.description && <p className="muted my-exam-desc">{e.description}</p>}

                  {/* Fenêtre de disponibilité */}
                  {(e.disponible_de || e.disponible_jusqu_a) && (
                    <div className="my-exam-dispo">
                      {e.disponible_de && <span><i className="bi bi-calendar-event" /> Dès le {fmtDateTime(e.disponible_de)}</span>}
                      {e.disponible_jusqu_a && <span><i className="bi bi-lock" /> Jusqu&apos;au {fmtDateTime(e.disponible_jusqu_a)}</span>}
                    </div>
                  )}

                  <div className="my-exam-meta">
                    <span className="muted"><i className="bi bi-clock" /> {e.duree_min} min</span>
                    <span className="muted"><i className="bi bi-calendar-event" /> Créé le {fmtDate(e.created_at)}</span>
                    {(e.tags || []).map((t) => <span key={t} className="tag">{t}</span>)}
                  </div>
                </div>

                {/* Colonne droite : progression sessions + actions */}
                <div className="my-exam-aside">
                  {resume && (
                    <div className={`sessions-progress${resume.tous_termines ? ' done' : ''}`}>
                      <span className="sessions-count">{resume.terminees}/{resume.total}</span>
                      <span className="sessions-label">
                        {resume.tous_termines ? <><i className="bi bi-check-circle-fill" /> Tous ont terminé</> : 'ont terminé'}
                      </span>
                      {resume.tous_termines && (
                        <p className="muted" style={{ fontSize: 12, marginTop: 4 }}>
                          Vérifiez les sessions de chaque étudiant.
                        </p>
                      )}
                    </div>
                  )}
                  <div className="my-exam-actions">
                    <button className="btn btn-primary btn-small" onClick={() => navigate(`/examens/${e.id}/resultats`)}>
                      <i className="bi bi-bar-chart" /> Résultats
                    </button>
                    <button className="btn btn-ghost" onClick={() => navigate(`/examens/${e.id}/modifier`)}>
                      <i className="bi bi-pencil-square" /> Modifier
                    </button>
                    <button className="btn btn-ghost btn-danger-ghost"
                      disabled={deleting === e.id} onClick={() => supprimer(e)}>
                      {deleting === e.id ? '...' : <><i className="bi bi-trash" /> Supprimer</>}
                    </button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
