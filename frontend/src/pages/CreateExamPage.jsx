// Page « Créer / Modifier un sujet » — réservée au personnel (examinateur/admin).
// Props : editMode (bool) — si true, charge l'examen depuis l'URL :id et pré-remplit.
// La route /creer → création (POST), la route /examens/:id/modifier → édition (PATCH).
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import './CreateExamPage.css'

const MODALITES = [
  { id: 'qcm', icon: 'bi-list-check', label: 'QCM', desc: 'Questions à choix multiple, corrigées automatiquement.' },
  { id: 'cas_pratique', icon: 'bi-pencil-square', label: 'Cas pratique', desc: 'Réponses rédigées, évaluées par un pair ou un examinateur.' },
  { id: 'oral', icon: 'bi-mic-fill', label: 'Oral', desc: 'Épreuve en face-à-face : visio + timer synchronisé + barème.' },
]

const emptyQcm = () => ({ enonce: '', propositions: ['', ''], bonne: 0, points: 2 })
const emptyOuverte = () => ({ enonce: '', points: 10 })
const emptyCritere = () => ({ libelle: '', points_max: 5 })

// Convertit un datetime-local string en ISO (ou null si vide)
const toISO = (s) => s ? new Date(s).toISOString() : null
// Convertit un ISO en datetime-local string (pour l'input)
const toLocal = (iso) => {
  if (!iso) return ''
  const d = new Date(iso)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export default function CreateExamPage({ editMode = false }) {
  const navigate = useNavigate()
  const { id: examId } = useParams()
  const { user } = useAuth()

  const [loading, setLoading] = useState(editMode)
  const [titre, setTitre] = useState('')
  const [description, setDescription] = useState('')
  const [modalite, setModalite] = useState('qcm')
  const [type, setType] = useState('entrainement')
  const [duree, setDuree] = useState(30)
  const [tags, setTags] = useState('')
  const [disponibleDe, setDisponibleDe] = useState('')
  const [disponibleJusqua, setDisponibleJusqua] = useState('')

  const [qcms, setQcms] = useState([emptyQcm()])
  const [ouvertes, setOuvertes] = useState([emptyOuverte()])
  const [criteres, setCriteres] = useState([emptyCritere(), emptyCritere()])

  // Destinataires
  const [classes, setClasses] = useState([])
  const [etudiants, setEtudiants] = useState([])
  const [selClasses, setSelClasses] = useState([])
  const [selEtudiants, setSelEtudiants] = useState([])
  const [destMode, setDestMode] = useState('public')

  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [deleting, setDeleting] = useState(false)

  // Chargement en mode édition
  useEffect(() => {
    api('/classes').then(setClasses).catch(() => {})
    api('/etudiants').then(setEtudiants).catch(() => {})
    if (!editMode || !examId) { setLoading(false); return }

    api(`/examens/${examId}`).then((e) => {
      setTitre(e.titre || '')
      setDescription(e.description || '')
      setModalite(e.modalite || 'qcm')
      setType(e.type || 'entrainement')
      setDuree(e.duree_min || 30)
      setTags((e.tags || []).join(', '))
      setDisponibleDe(toLocal(e.disponible_de))
      setDisponibleJusqua(toLocal(e.disponible_jusqu_a))

      if (e.questions?.length) {
        if (e.modalite === 'qcm') {
          setQcms(e.questions.map((q) => ({
            enonce: q.enonce, propositions: q.propositions, bonne: q.bonne_reponse ?? 0, points: q.points
          })))
        } else if (e.modalite === 'cas_pratique') {
          setOuvertes(e.questions.map((q) => ({ enonce: q.enonce, points: q.points })))
        }
      }
      if (e.grille?.criteres?.length) {
        setCriteres(e.grille.criteres.map((c) => ({ libelle: c.libelle, points_max: c.points_max })))
      }
      if (e.assignations) {
        if (e.assignations.classes?.length) {
          setSelClasses(e.assignations.classes.map((c) => c.id))
          setDestMode('classes')
        } else if (e.assignations.etudiants?.length) {
          setSelEtudiants(e.assignations.etudiants.map((et) => et.id))
          setDestMode('etudiants')
        }
      }
    }).catch(() => setError('Impossible de charger l\'examen.'))
      .finally(() => setLoading(false))
  }, [editMode, examId])

  const updateAt = (list, setList) => (i, patch) =>
    setList(list.map((item, j) => (j === i ? { ...item, ...patch } : item)))
  const removeAt = (list, setList) => (i) =>
    setList(list.filter((_, j) => j !== i))

  const setQcm = updateAt(qcms, setQcms)
  const setOuverte = updateAt(ouvertes, setOuvertes)
  const setCritere = updateAt(criteres, setCriteres)

  function toggleClasse(id) {
    setSelClasses((p) => p.includes(id) ? p.filter((c) => c !== id) : [...p, id])
  }
  function toggleEtudiant(id) {
    setSelEtudiants((p) => p.includes(id) ? p.filter((e) => e !== id) : [...p, id])
  }

  function validate() {
    if (!titre.trim()) return 'Le titre est requis.'
    if (modalite === 'qcm') {
      if (qcms.length === 0) return 'Ajoutez au moins une question.'
      for (const [i, q] of qcms.entries()) {
        if (!q.enonce.trim()) return `Question ${i + 1} : l'énoncé est vide.`
        if (q.propositions.filter((p) => p.trim()).length < 2) return `Question ${i + 1} : il faut au moins 2 propositions.`
        if (!q.propositions[q.bonne]?.trim()) return `Question ${i + 1} : choisissez la bonne réponse.`
      }
    }
    if (modalite === 'cas_pratique') {
      if (ouvertes.length === 0) return 'Ajoutez au moins une question.'
      for (const [i, q] of ouvertes.entries()) {
        if (!q.enonce.trim()) return `Question ${i + 1} : l'énoncé est vide.`
      }
    }
    if (modalite === 'oral' && !criteres.some((c) => c.libelle.trim()))
      return 'Ajoutez au moins un critère au barème.'
    if (destMode === 'classes' && selClasses.length === 0)
      return 'Sélectionnez au moins une classe ou choisissez "Public".'
    if (destMode === 'etudiants' && selEtudiants.length === 0)
      return 'Sélectionnez au moins un étudiant ou choisissez "Public".'
    if (disponibleDe && disponibleJusqua && new Date(disponibleDe) >= new Date(disponibleJusqua))
      return 'La date de fin doit être après la date de début.'
    return ''
  }

  async function submit(e) {
    e.preventDefault()
    const msg = validate()
    if (msg) { setError(msg); return }
    setError(''); setBusy(true)

    const body = {
      titre: titre.trim(), description: description.trim(),
      type, modalite, duree_min: Number(duree) || 30,
      tags: tags.split(',').map((t) => t.trim()).filter(Boolean),
      questions: [],
      disponible_de: toISO(disponibleDe),
      disponible_jusqu_a: toISO(disponibleJusqua),
    }

    if (modalite === 'qcm') {
      body.questions = qcms.map((q) => ({
        enonce: q.enonce.trim(), type: 'qcm',
        propositions: q.propositions.filter((p) => p.trim()),
        bonne_reponse: q.bonne, points: Number(q.points) || 1,
      }))
    } else if (modalite === 'cas_pratique') {
      body.questions = ouvertes.map((q) => ({
        enonce: q.enonce.trim(), type: 'cas_pratique', points: Number(q.points) || 10,
      }))
    } else if (modalite === 'oral') {
      body.grille = {
        titre: `Grille — ${titre.trim()}`,
        criteres: criteres.filter((c) => c.libelle.trim())
          .map((c) => ({ libelle: c.libelle.trim(), points_max: Number(c.points_max) || 5 })),
      }
    }

    if (destMode === 'classes' && selClasses.length > 0)
      body.assignations = { classe_ids: selClasses, utilisateur_ids: [] }
    else if (destMode === 'etudiants' && selEtudiants.length > 0)
      body.assignations = { classe_ids: [], utilisateur_ids: selEtudiants }

    try {
      if (editMode && examId) {
        await api(`/examens/${examId}`, { method: 'PATCH', body })
        navigate('/mes-sujets', { replace: true })
      } else {
        await api('/examens', { method: 'POST', body })
        navigate('/', { replace: true })
      }
    } catch (err) {
      setError(err.message)
      setBusy(false)
    }
  }

  async function supprimerExamen() {
    if (!window.confirm(`Supprimer définitivement « ${titre} » ?\n\nToutes les sessions et évaluations associées seront également supprimées.`)) return
    setDeleting(true)
    try {
      await api(`/examens/${examId}`, { method: 'DELETE' })
      navigate('/mes-sujets', { replace: true })
    } catch (err) {
      setError(err.message)
      setDeleting(false)
    }
  }

  if (loading) return <div className="center" style={{ padding: 80 }}><div className="spinner" /></div>

  return (
    <div className="create-exam">
      <div className="page-hero">
        <h1>{editMode ? 'Modifier le sujet' : 'Créer un sujet'}</h1>
        <p className="muted">
          {editMode
            ? 'Modifiez les informations, questions et destinataires de ce sujet.'
            : 'Créez un sujet d\'entraînement ou un examen officiel pour vos étudiants.'}
        </p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <form onSubmit={submit}>
        {/* Étape 1 : informations générales */}
        <div className="card create-section">
          <h2 className="create-step">1 · Informations générales</h2>
          <div className="field">
            <label>Titre *</label>
            <input className="input" value={titre} onChange={(e) => setTitre(e.target.value)}
              placeholder="Ex : QCM Bases de données relationnelles" />
          </div>
          <div className="field">
            <label>Description</label>
            <textarea className="textarea" rows={2} value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Décrivez brièvement le contenu et les objectifs du sujet..." />
          </div>
          <div className="create-row">
            <div className="field">
              <label>Type</label>
              <select className="select" value={type} onChange={(e) => setType(e.target.value)}>
                <option value="entrainement">Entraînement</option>
                <option value="officiel">Examen officiel</option>
              </select>
            </div>
            <div className="field">
              <label>Durée (minutes)</label>
              <input className="input" type="number" min="1" max="480" value={duree}
                onChange={(e) => setDuree(e.target.value)} />
            </div>
            <div className="field" style={{ flex: 1 }}>
              <label>Tags <span className="muted">(virgules)</span></label>
              <input className="input" value={tags} onChange={(e) => setTags(e.target.value)}
                placeholder="Ex : SQL, Bases de données" />
            </div>
          </div>
          {/* Fenêtre de disponibilité */}
          <div className="create-row dispo-row">
            <div className="field">
              <label><i className="bi bi-calendar-event" /> Disponible à partir du <span className="muted">(optionnel)</span></label>
              <input className="input" type="datetime-local"
                value={disponibleDe} onChange={(e) => setDisponibleDe(e.target.value)} />
            </div>
            <div className="field">
              <label><i className="bi bi-lock" /> Disponible jusqu'au <span className="muted">(optionnel)</span></label>
              <input className="input" type="datetime-local"
                value={disponibleJusqua} onChange={(e) => setDisponibleJusqua(e.target.value)} />
            </div>
          </div>
          {(disponibleDe || disponibleJusqua) && (
            <p className="muted dispo-hint">
              <i className="bi bi-info-circle" /> Hors de cette fenêtre, les étudiants ne verront pas l&apos;examen.
              Laissez vide pour qu&apos;il soit toujours accessible.
            </p>
          )}
        </div>

        {/* Étape 2 : modalité */}
        <div className="card create-section">
          <h2 className="create-step">2 · Type d&apos;épreuve</h2>
          <div className="modalite-cards">
            {MODALITES.map((m) => (
              <button key={m.id} type="button"
                className={`modalite-card ${modalite === m.id ? 'selected' : ''}`}
                onClick={() => setModalite(m.id)}>
                <i className={`modalite-icon bi ${m.icon}`} />
                <strong>{m.label}</strong>
                <span className="muted">{m.desc}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Étape 3 : contenu */}
        <div className="card create-section">
          {modalite === 'qcm' && (
            <>
              <h2 className="create-step">3 · Questions du QCM</h2>
              {qcms.map((q, i) => (
                <div key={i} className="question-editor">
                  <div className="question-editor-head">
                    <strong>Question {i + 1}</strong>
                    <div className="question-editor-actions">
                      <label className="points-label">Points
                        <input className="input input-points" type="number" min="0.5" step="0.5"
                          value={q.points} onChange={(e) => setQcm(i, { points: e.target.value })} />
                      </label>
                      {qcms.length > 1 && (
                        <button type="button" className="btn btn-ghost btn-small"
                          onClick={() => removeAt(qcms, setQcms)(i)}><i className="bi bi-trash" /></button>
                      )}
                    </div>
                  </div>
                  <input className="input" value={q.enonce}
                    onChange={(e) => setQcm(i, { enonce: e.target.value })}
                    placeholder="Énoncé de la question..." />
                  <div className="props">
                    <p className="muted props-hint">Cochez la bonne réponse :</p>
                    {q.propositions.map((p, j) => (
                      <div key={j} className="prop-row">
                        <input type="radio" name={`bonne-${i}`} checked={q.bonne === j}
                          onChange={() => setQcm(i, { bonne: j })} title="Bonne réponse" />
                        <input className="input" value={p}
                          onChange={(e) => {
                            const props = [...q.propositions]; props[j] = e.target.value
                            setQcm(i, { propositions: props })
                          }} placeholder={`Proposition ${j + 1}`} />
                        {q.propositions.length > 2 && (
                          <button type="button" className="btn btn-ghost btn-small"
                            onClick={() => {
                              const props = q.propositions.filter((_, k) => k !== j)
                              setQcm(i, { propositions: props, bonne: q.bonne >= props.length ? 0 : q.bonne })
                            }}><i className="bi bi-x-lg" /></button>
                        )}
                      </div>
                    ))}
                    <button type="button" className="btn btn-ghost btn-small"
                      onClick={() => setQcm(i, { propositions: [...q.propositions, ''] })}>
                      <i className="bi bi-plus-lg" /> Ajouter une proposition
                    </button>
                  </div>
                </div>
              ))}
              <button type="button" className="btn btn-ghost"
                onClick={() => setQcms([...qcms, emptyQcm()])}><i className="bi bi-plus-lg" /> Ajouter une question</button>
            </>
          )}

          {modalite === 'cas_pratique' && (
            <>
              <h2 className="create-step">3 · Questions du cas pratique</h2>
              {ouvertes.map((q, i) => (
                <div key={i} className="question-editor">
                  <div className="question-editor-head">
                    <strong>Question {i + 1}</strong>
                    <div className="question-editor-actions">
                      <label className="points-label">Points
                        <input className="input input-points" type="number" min="1" step="1"
                          value={q.points} onChange={(e) => setOuverte(i, { points: e.target.value })} />
                      </label>
                      {ouvertes.length > 1 && (
                        <button type="button" className="btn btn-ghost btn-small"
                          onClick={() => removeAt(ouvertes, setOuvertes)(i)}><i className="bi bi-trash" /></button>
                      )}
                    </div>
                  </div>
                  <textarea className="textarea" rows={3} value={q.enonce}
                    onChange={(e) => setOuverte(i, { enonce: e.target.value })}
                    placeholder="Énoncé complet du cas pratique..." />
                </div>
              ))}
              <button type="button" className="btn btn-ghost"
                onClick={() => setOuvertes([...ouvertes, emptyOuverte()])}><i className="bi bi-plus-lg" /> Ajouter une question</button>
            </>
          )}

          {modalite === 'oral' && (
            <>
              <h2 className="create-step">3 · Grille de correction (barème)</h2>
              <p className="muted" style={{ marginBottom: 16 }}>
                Ces critères seront notés en direct par l&apos;examinateur pendant l&apos;oral.
              </p>
              {criteres.map((c, i) => (
                <div key={i} className="critere-editor">
                  <input className="input" value={c.libelle}
                    onChange={(e) => setCritere(i, { libelle: e.target.value })}
                    placeholder={`Critère ${i + 1} — ex : Clarté de la présentation`} />
                  <label className="points-label">
                    / <input className="input input-points" type="number" min="1" step="1"
                      value={c.points_max}
                      onChange={(e) => setCritere(i, { points_max: e.target.value })} /> pts
                  </label>
                  {criteres.length > 1 && (
                    <button type="button" className="btn btn-ghost btn-small"
                      onClick={() => removeAt(criteres, setCriteres)(i)}><i className="bi bi-trash" /></button>
                  )}
                </div>
              ))}
              <button type="button" className="btn btn-ghost"
                onClick={() => setCriteres([...criteres, emptyCritere()])}><i className="bi bi-plus-lg" /> Ajouter un critère</button>
            </>
          )}
        </div>

        {/* Étape 4 : Destinataires */}
        <div className="card create-section">
          <h2 className="create-step">4 · Destinataires</h2>
          <p className="muted" style={{ marginBottom: 16 }}>
            Par défaut visible par tous les étudiants. Vous pouvez restreindre à des classes ou étudiants précis.
          </p>
          <div className="dest-tabs">
            {[
              { key: 'public', icon: 'bi-globe', label: 'Public (tous les étudiants)' },
              { key: 'classes', icon: 'bi-building', label: `Par classe${selClasses.length ? ` (${selClasses.length})` : ''}` },
              { key: 'etudiants', icon: 'bi-person', label: `Par étudiant${selEtudiants.length ? ` (${selEtudiants.length})` : ''}` },
            ].map((t) => (
              <button key={t.key} type="button"
                className={`dest-tab${destMode === t.key ? ' active' : ''}`}
                onClick={() => setDestMode(t.key)}><i className={`bi ${t.icon}`} /> {t.label}</button>
            ))}
          </div>
          {destMode === 'classes' && (
            <div className="dest-grid">
              {classes.length === 0 && <p className="muted">Aucune classe disponible.</p>}
              {classes.map((c) => (
                <label key={c.id} className={`dest-item${selClasses.includes(c.id) ? ' selected' : ''}`}>
                  <input type="checkbox" checked={selClasses.includes(c.id)}
                    onChange={() => toggleClasse(c.id)} />
                  <span><strong>{c.nom}</strong>
                    <span className="muted"> · {c.membres?.length ?? 0} étudiant{c.membres?.length !== 1 ? 's' : ''}</span>
                  </span>
                </label>
              ))}
            </div>
          )}
          {destMode === 'etudiants' && (
            <div className="dest-grid">
              {etudiants.length === 0 && <p className="muted">Aucun étudiant disponible.</p>}
              {etudiants.map((e) => (
                <label key={e.id} className={`dest-item${selEtudiants.includes(e.id) ? ' selected' : ''}`}>
                  <input type="checkbox" checked={selEtudiants.includes(e.id)}
                    onChange={() => toggleEtudiant(e.id)} />
                  <span><strong>{e.prenom} {e.nom}</strong>
                    <span className="muted"> · {e.email}</span>
                  </span>
                </label>
              ))}
            </div>
          )}
        </div>

        {/* Validation */}
        <div className="create-submit">
          {editMode && (
            <button type="button" className="btn btn-danger" disabled={deleting}
              onClick={supprimerExamen}>
              {deleting ? 'Suppression...' : <><i className="bi bi-trash" /> Supprimer cet examen</>}
            </button>
          )}
          <button type="button" className="btn btn-ghost"
            onClick={() => navigate(editMode ? '/mes-sujets' : '/')}>Annuler</button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? (editMode ? 'Mise à jour...' : 'Création...') : (editMode ? 'Enregistrer les modifications' : 'Créer le sujet')}
          </button>
        </div>
      </form>
    </div>
  )
}
