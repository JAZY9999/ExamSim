// Page 4 : Interface Épreuve Orale (Timer F2F & Barème).
// Écran divisé en deux : à gauche la grille d'évaluation (examinateur),
// à droite le flux vidéo (prestataire Jitsi) + le chronomètre synchronisé.
//
// Le chronomètre est synchronisé en temps réel via WebSocket : l'examinateur
// contrôle (start/pause/reset), le serveur diffuse l'état à tous les
// participants chaque seconde. C'est le cœur technique du projet.
import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import './OralPage.css'

export default function OralPage() {
  const { id: sessionId } = useParams()
  const { user } = useAuth()
  const navigate = useNavigate()
  const isExaminateur = user?.role === 'examinateur' || user?.role === 'admin'

  const [examen, setExamen] = useState(null)
  const [etudiant, setEtudiant] = useState(null)
  const [jitsiDomain, setJitsiDomain] = useState('framatalk.org')
  const [seconds, setSeconds] = useState(0)
  const [running, setRunning] = useState(false)
  const [connected, setConnected] = useState(false)

  // État de l'évaluation (côté examinateur).
  const [notes, setNotes] = useState({})   // critereId -> points
  const [remarques, setRemarques] = useState('')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  // Note finale : reçue en direct via WebSocket (étudiant) ou après
  // enregistrement (examinateur). {note, max}
  const [evalResult, setEvalResult] = useState(null)

  const wsRef = useRef(null)

  // Charge l'examen (grille) + le candidat + la config publique (domaine Jitsi).
  useEffect(() => {
    api(`/sessions/${sessionId}`)
      .then(({ examen, etudiant }) => { setExamen(examen); setEtudiant(etudiant) })
      .catch((e) => setError(e.message))
    api('/config').then((c) => setJitsiDomain(c.jitsi_domain)).catch(() => {})
  }, [sessionId])

  // Connexion WebSocket au timer synchronisé.
  useEffect(() => {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${window.location.host}/ws/oral/${sessionId}`)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      // L'examinateur initialise la durée si l'examen est chargé.
    }
    ws.onmessage = (evt) => {
      const msg = JSON.parse(evt.data)
      if (msg.type === 'state') {
        setSeconds(msg.seconds)
        setRunning(msg.running)
      }
      // La note validée par l'examinateur arrive en direct chez l'étudiant.
      if (msg.type === 'evaluated') {
        setEvalResult({ note: msg.note, max: msg.note_max })
      }
    }
    ws.onclose = () => setConnected(false)
    return () => ws.close()
  }, [sessionId])

  // Une fois l'examen chargé, l'examinateur pousse la durée initiale (si à 0).
  useEffect(() => {
    if (examen && isExaminateur && wsRef.current?.readyState === WebSocket.OPEN && seconds === 0 && !running) {
      send({ type: 'set', seconds: (examen.duree_min || 15) * 60 })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [examen, connected])

  function send(msg) {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }

  if (error) return <div className="run-error">{error}</div>
  if (!examen) return <div className="center" style={{ height: '100vh' }}><div className="spinner" /></div>

  const grille = examen.grille
  const critical = seconds <= 60 && seconds > 0
  const totalObtenu = Object.values(notes).reduce((a, b) => a + Number(b || 0), 0)
  const totalMax = (grille?.criteres || []).reduce((a, c) => a + c.points_max, 0)

  // Nom de salle Jitsi déterministe : les deux participants ouvrent la MÊME
  // session (l'examinateur rejoint via « Oraux en cours »), donc la même salle.
  const roomName = `ExamSim-Oral-${sessionId}`
  // On transmet le vrai nom du participant à Jitsi (avec son rôle), pour que
  // chacun voie clairement qui est l'examinateur dans l'appel.
  const displayName = `${user?.prenom || ''} ${user?.nom || ''}${isExaminateur ? ' (Examinateur)' : ''}`.trim()
  const jitsiUrl = `https://${jitsiDomain}/${roomName}`
    + `#config.prejoinPageEnabled=false`
    + `&userInfo.displayName=${encodeURIComponent(`"${displayName}"`)}`

  async function enregistrer() {
    setError('')
    try {
      const payload = {
        remarques,
        notes: (grille?.criteres || []).map((c) => ({
          critere_id: c.id,
          points: Number(notes[c.id] || 0),
        })),
      }
      await api(`/sessions/${sessionId}/evaluations`, { method: 'POST', body: payload })
      const total = payload.notes.reduce((a, n) => a + n.points, 0)
      // Annonce la note à toute la salle : l'étudiant la voit instantanément.
      send({ type: 'evaluated', note: total, note_max: totalMax })
      setEvalResult({ note: total, max: totalMax })
      setSaved(true)
    } catch (e) {
      setError(e.message)
    }
  }

  return (
    <div className="oral">
      {/* Bandeau supérieur */}
      <div className="oral-topbar">
        <button className="btn btn-ghost" onClick={() => navigate('/')}><i className="bi bi-chevron-left" /> Quitter</button>
        <div className="oral-title">{examen.titre}</div>
        <span className={`conn-badge ${connected ? 'on' : 'off'}`}>
          <i className={`bi ${connected ? 'bi-record-circle-fill' : 'bi-record-circle'}`} /> {connected ? 'Synchronisé' : 'Déconnecté'}
        </span>
      </div>

      <div className="oral-split">
        {/* Moitié gauche : grille d'évaluation (examinateur) */}
        <div className="oral-left">
          {isExaminateur && saved ? (
            /* --- Écran de résultat côté examinateur --- */
            <div className="eval-done">
              <div className="eval-done-icon"><i className="bi bi-check-circle-fill" /></div>
              <h2>Évaluation enregistrée</h2>
              <p>
                <strong>{etudiant?.prenom} {etudiant?.nom}</strong> est passé(e) à
                l'épreuve « {examen.titre} » et a obtenu :
              </p>
              <div className="eval-note-big">
                {evalResult?.note?.toFixed(1)} <span>/ {evalResult?.max}</span>
              </div>
              {remarques && <p className="muted eval-remarques">« {remarques} »</p>}
              <p className="muted" style={{ fontSize: 13 }}>
                La note a été transmise en direct à l'étudiant et enregistrée
                dans son historique.
              </p>
              <button className="btn btn-primary" onClick={() => navigate('/')}>
                Retour à l'accueil
              </button>
            </div>
          ) : isExaminateur ? (
            <>
              <h2 className="section-title">Grille d'évaluation</h2>
              {error && <div className="alert alert-error">{error}</div>}

              {/* Candidat évalué : détecté depuis la session, vérifié en base.
                  Volontairement non modifiable — la note est rattachée de façon
                  sûre au compte de l'étudiant qui passe CETTE session. */}
              {etudiant && (
                <div className="field">
                  <label>Candidat évalué (compte vérifié)</label>
                  <input className="input" disabled
                    value={`${etudiant.prenom} ${etudiant.nom} — ${etudiant.email}`} />
                </div>
              )}

              {grille ? (
                <>
                  <div className="criteres">
                    {grille.criteres.map((c) => (
                      <div key={c.id} className="critere-row">
                        <div className="critere-head">
                          <span>{c.libelle}</span>
                          <span className="critere-pts">
                            {notes[c.id] || 0} / {c.points_max}
                          </span>
                        </div>
                        <input type="range" min="0" max={c.points_max} step="0.5"
                          value={notes[c.id] || 0}
                          onChange={(e) => { setSaved(false); setNotes({ ...notes, [c.id]: e.target.value }) }}
                          className="slider" />
                      </div>
                    ))}
                  </div>

                  <div className="note-totale">
                    Note totale : <strong>{totalObtenu.toFixed(1)} / {totalMax}</strong>
                  </div>

                  <div className="field" style={{ marginTop: 16 }}>
                    <label>Remarques</label>
                    <textarea className="textarea" value={remarques}
                      onChange={(e) => { setSaved(false); setRemarques(e.target.value) }}
                      placeholder="Observations sur la prestation orale..." />
                  </div>

                  <button className="btn btn-primary btn-block" onClick={enregistrer}>
                    Enregistrer l'évaluation
                  </button>
                </>
              ) : (
                <p className="muted">Cet examen n'a pas de grille de correction.</p>
              )}
            </>
          ) : evalResult ? (
            /* --- La note arrive en direct chez l'étudiant --- */
            <div className="eval-done">
              <div className="eval-done-icon"><i className="bi bi-check-circle-fill" /></div>
              <h2>Épreuve terminée</h2>
              <p>L'examinateur a validé votre note :</p>
              <div className="eval-note-big">
                {evalResult.note?.toFixed(1)} <span>/ {evalResult.max}</span>
              </div>
              <p className="muted" style={{ fontSize: 13 }}>
                Retrouvez le détail dans vos statistiques.
              </p>
              <button className="btn btn-primary" onClick={() => navigate('/statistiques')}>
                Voir mes statistiques
              </button>
            </div>
          ) : (
            <div className="student-view">
              <h2 className="section-title">Épreuve orale en cours</h2>
              <p className="muted">
                Vous passez votre oral. L'examinateur contrôle le chronomètre
                affiché à droite. Restez concentré et gérez votre temps de parole.
              </p>
              <div className="student-tips">
                <p><i className="bi bi-lightbulb" /> Le chronomètre passe au rouge dans la dernière minute.</p>
                <p><i className="bi bi-camera-video" /> Vérifiez que votre caméra et votre micro sont actifs.</p>
                <p><i className="bi bi-bullseye" /> Votre note s'affichera ici dès que l'examinateur l'aura validée.</p>
              </div>
            </div>
          )}
        </div>

        {/* Moitié droite : visio + chronomètre */}
        <div className="oral-right">
          {/* Informations sur le candidat (vue examinateur) */}
          {isExaminateur && etudiant && (
            <div className="candidat-info">
              <span className="candidat-avatar">
                {(etudiant.prenom?.[0] || '') + (etudiant.nom?.[0] || '')}
              </span>
              <div>
                <strong>{etudiant.prenom} {etudiant.nom}</strong>
                <div className="muted" style={{ fontSize: 13 }}>{etudiant.email} · Candidat</div>
              </div>
            </div>
          )}

          {/* Chronomètre synchronisé */}
          <div className={`oral-timer ${critical ? 'critical' : ''} ${seconds === 0 && !running ? 'ended' : ''}`}>
            <div className="oral-timer-value">{fmt(seconds)}</div>
            <div className="oral-timer-label">
              {seconds === 0 ? 'Temps écoulé' : running ? 'En cours' : 'En pause'}
            </div>
          </div>

          {/* Contrôles réservés à l'examinateur */}
          {isExaminateur && (
            <div className="oral-controls">
              {!running ? (
                <button className="btn btn-success" onClick={() => send({ type: 'start' })}><i className="bi bi-play-fill" /> Start</button>
              ) : (
                <button className="btn btn-ghost" onClick={() => send({ type: 'pause' })}><i className="bi bi-pause-fill" /> Pause</button>
              )}
              <button className="btn btn-danger"
                onClick={() => send({ type: 'reset', seconds: (examen.duree_min || 15) * 60 })}>
                <i className="bi bi-arrow-counterclockwise" /> Reset
              </button>
            </div>
          )}

          {/* Flux vidéo — prestataire extérieur Jitsi */}
          <div className="oral-video">
            <iframe
              title="Visioconférence Jitsi"
              src={jitsiUrl}
              allow="camera; microphone; fullscreen; display-capture; autoplay"
            />
          </div>
          <p className="video-hint muted">
            Visioconférence fournie par <strong>{jitsiDomain}</strong> (prestataire extérieur).
            {' '}Si la vidéo ne s'affiche pas,{' '}
            <a href={jitsiUrl} target="_blank" rel="noreferrer">
              ouvrez-la dans une fenêtre séparée
            </a>.
          </p>
        </div>
      </div>
    </div>
  )
}

function fmt(s) {
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`
}
