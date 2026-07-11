// Page 3 : Interface de passage d'examen (Écrit / QCM).
// En-tête fixe (timer + progression), zone de question au centre, navigation en bas.
import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import './ExamRunPage.css'

export default function ExamRunPage() {
  const { id: sessionId } = useParams() // ici l'id est celui de la SESSION
  const navigate = useNavigate()

  const [examen, setExamen] = useState(null)
  const [current, setCurrent] = useState(0)
  const [answers, setAnswers] = useState({}) // questionId -> { choix | texte }
  const [seconds, setSeconds] = useState(0)
  const [result, setResult] = useState(null)
  const [error, setError] = useState('')
  const timerRef = useRef(null)

  // Chargement de la session + examen.
  useEffect(() => {
    api(`/sessions/${sessionId}`)
      .then(({ session, examen }) => {
        setExamen(examen)
        setSeconds(session.temps_restant)
      })
      .catch((e) => setError(e.message))
  }, [sessionId])

  // Chronomètre décroissant.
  useEffect(() => {
    if (!examen || result) return
    timerRef.current = setInterval(() => {
      setSeconds((s) => {
        if (s <= 1) {
          clearInterval(timerRef.current)
          handleSubmit(true) // soumission automatique à l'expiration
          return 0
        }
        return s - 1
      })
    }, 1000)
    return () => clearInterval(timerRef.current)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [examen, result])

  if (error) return <div className="run-error">{error}</div>
  if (!examen) return <div className="center" style={{ height: '100vh' }}><div className="spinner" /></div>

  const questions = examen.questions || []
  const q = questions[current]
  const total = questions.length
  const progress = total ? Math.round(((current + 1) / total) * 100) : 0
  const critical = seconds <= 300 // < 5 min => rouge

  function setAnswer(value) {
    setAnswers({ ...answers, [q.id]: value })
  }

  async function handleSubmit(auto = false) {
    if (!auto && !confirm('Soumettre définitivement l\'examen ?')) return
    clearInterval(timerRef.current)
    const reponses = questions.map((qq) => ({
      question_id: qq.id,
      choix: answers[qq.id]?.choix ?? null,
      texte: answers[qq.id]?.texte ?? '',
    }))
    try {
      const res = await api(`/sessions/${sessionId}/submit`, {
        method: 'POST',
        body: { reponses },
      })
      setResult(res)
    } catch (e) {
      setError(e.message)
    }
  }

  // Écran de résultat.
  if (result) {
    return (
      <div className="run-result">
        <div className="card result-card">
          <div className="result-check">✅</div>
          <h1>Examen soumis !</h1>
          {result.score_auto != null ? (
            <p className="result-score">
              Note automatique : <strong>{result.score_auto.toFixed(1)} / 20</strong>
            </p>
          ) : (
            <p className="muted">Votre copie sera corrigée (cas pratique / peer-to-peer).</p>
          )}
          <button className="btn btn-primary" onClick={() => navigate('/statistiques')}>
            Voir mes statistiques
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="run">
      {/* En-tête fixe : timer + progression */}
      <header className="run-header">
        <div className="run-title">{examen.titre}</div>
        <div className="run-progress">
          <span>Question {current + 1}/{total}</span>
          <div className="progress-bar"><div className="progress-fill" style={{ width: `${progress}%` }} /></div>
        </div>
        <div className={`run-timer ${critical ? 'critical' : ''}`}>
          ⏱ {fmt(seconds)}
        </div>
      </header>

      {/* Zone centrale : la question */}
      <main className="run-main">
        {q && (
          <div className="question-card">
            <div className="question-num">Question {current + 1}</div>
            <p className="question-enonce">{q.enonce}</p>

            {q.type === 'qcm' ? (
              <div className="options">
                {q.propositions.map((prop, i) => (
                  <label key={i} className={`option ${answers[q.id]?.choix === i ? 'selected' : ''}`}>
                    <input type="radio" name={q.id}
                      checked={answers[q.id]?.choix === i}
                      onChange={() => setAnswer({ choix: i })} />
                    <span>{prop}</span>
                  </label>
                ))}
              </div>
            ) : (
              <textarea className="textarea" rows={10}
                placeholder="Rédigez votre réponse ici..."
                value={answers[q.id]?.texte || ''}
                onChange={(e) => setAnswer({ texte: e.target.value })} />
            )}
          </div>
        )}
      </main>

      {/* Navigation bas de page */}
      <footer className="run-footer">
        <button className="btn btn-ghost" disabled={current === 0}
          onClick={() => setCurrent((c) => c - 1)}>← Précédente</button>

        {current < total - 1 ? (
          <button className="btn btn-ghost" onClick={() => setCurrent((c) => c + 1)}>
            Suivante →
          </button>
        ) : <span />}

        <button className="btn btn-primary run-submit" onClick={() => handleSubmit(false)}>
          Soumettre l'examen
        </button>
      </footer>
    </div>
  )
}

function fmt(s) {
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`
}
