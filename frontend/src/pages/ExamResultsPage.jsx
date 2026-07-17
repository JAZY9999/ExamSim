// Page des résultats détaillés d'un examen (style Socrative)
import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import './PageCommon.css'
import './ExamResultsPage.css'

export default function ExamResultsPage() {
  const { id: examId } = useParams()
  const navigate = useNavigate()

  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Toggles de contrôle (Socrative style)
  const [showNames, setShowNames] = useState(true)
  const [showResponses, setShowResponses] = useState(true)
  const [showResults, setShowResults] = useState(true)

  useEffect(() => {
    api(`/examens/${examId}/resultats`)
      .then(setData)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [examId])

  if (loading) return <div className="center" style={{ padding: 80 }}><div className="spinner" /></div>
  if (error) return <div className="alert alert-error">{error}</div>
  if (!data) return <div className="alert alert-error">Aucune donnée trouvée.</div>

  const { examen, questions, resultats } = data
  const isOral = examen.modalite === 'oral'
  const criteres = examen.grille?.criteres || []
  const columns = isOral ? criteres : questions

  // Calcule la note finale pour l'affichage (sur 20 ou %)
  function getScoreDisplay(res) {
    const score = res.note_evaluation !== null ? res.note_evaluation : res.score_auto
    if (score === null || score === undefined) return '—'
    const percentage = Math.round((score / 20) * 100)
    return `${percentage}%`
  }

  // Calcule le taux de réussite d'une question/critère pour toute la classe
  function getColumnStats(colId) {
    if (resultats.length === 0) return '0%'
    let correctCount = 0
    let evaluatedCount = 0

    resultats.forEach((res) => {
      if (isOral) {
        // Pour l'oral: moyenne des points sur ce critère
        const crit = res.criteres?.[colId]
        if (crit) {
          correctCount += crit.Points
          evaluatedCount += crit.PointsMax
        }
      } else {
        // Pour les autres examens
        const rep = res.reponses[colId]
        if (!rep) return
        const isQcm = questions.find((q) => q.id === colId)?.type === 'qcm'
        if (isQcm) {
          if (rep.correct) correctCount++
        } else {
          if (rep.texte && rep.texte.trim()) correctCount++
        }
        evaluatedCount++
      }
    })

    if (isOral) {
      if (evaluatedCount === 0) return '0%'
      return `${Math.round((correctCount / evaluatedCount) * 100)}%`
    } else {
      const total = isOral ? criteres.length : questions.length
      if (resultats.length === 0) return '0%'
      return `${Math.round((correctCount / resultats.length) * 100)}%`
    }
  }

  return (
    <div className="exam-results-page">
      <div className="results-header-container">
        <button className="btn btn-ghost" onClick={() => navigate('/mes-sujets')}>
          <i className="bi bi-chevron-left" /> Retour aux sujets
        </button>
        <div className="results-title-section">
          <h1>{examen.titre}</h1>
          <span className="tag tag-info">
            {examen.modalite === 'qcm' ? 'QCM' : examen.modalite === 'oral' ? 'Oral' : 'Cas pratique'}
          </span>
        </div>
      </div>

      {/* Barre de contrôles Socrative */}
      <div className="card socrative-controls">
        <div className="control-toggle">
          <label className="switch">
            <input
              type="checkbox"
              checked={showNames}
              onChange={(e) => setShowNames(e.target.checked)}
            />
            <span className="slider round"></span>
          </label>
          <span className="control-label">Afficher les noms</span>
        </div>

        <div className="control-toggle">
          <label className="switch">
            <input
              type="checkbox"
              checked={showResponses}
              onChange={(e) => setShowResponses(e.target.checked)}
            />
            <span className="slider round"></span>
          </label>
          <span className="control-label">Afficher les réponses</span>
        </div>

        <div className="control-toggle">
          <label className="switch">
            <input
              type="checkbox"
              checked={showResults}
              onChange={(e) => setShowResults(e.target.checked)}
            />
            <span className="slider round"></span>
          </label>
          <span className="control-label">Afficher les couleurs</span>
        </div>
      </div>

      {/* Grille Socrative */}
      <div className="card socrative-table-card">
        <div className="table-responsive">
          <table className="socrative-table">
            <thead>
              <tr>
                <th className="th-student-name">NOM</th>
                <th className="th-student-score">SCORE</th>
                {columns.map((col, idx) => (
                  <th key={col.id} className="th-question" title={isOral ? col.libelle : col.enonce}>
                    {isOral ? `C${idx + 1}` : idx + 1}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {resultats.map((res, index) => {
                const scoreStr = getScoreDisplay(res)
                return (
                  <tr key={res.session_id} className="socrative-row">
                    <td
                      className="td-student-name clickable"
                      onClick={() => navigate(`/sessions/${res.session_id}`)}
                      title="Cliquez pour ouvrir la copie"
                    >
                      {showNames ? res.etudiant_nom : `Étudiant ${index + 1}`}
                      <span className="student-email">{res.etudiant_email}</span>
                    </td>
                    <td
                      className={`td-student-score clickable ${showResults ? 'score-highlight' : ''}`}
                      onClick={() => navigate(`/sessions/${res.session_id}`)}
                    >
                      {scoreStr}
                    </td>

                    {/* Réponses / Notes pour chaque critère ou question */}
                    {columns.map((col) => {
                      let cellClass = ''
                      let cellContent = '—'

                      if (isOral) {
                        const crit = res.criteres?.[col.id]
                        if (crit) {
                          const scoreRatio = crit.Points / crit.PointsMax
                          if (showResults) {
                            cellClass = scoreRatio >= 0.8 ? 'cell-correct' : scoreRatio >= 0.5 ? 'cell-answered' : 'cell-incorrect'
                          }
                          cellContent = showResponses ? `${crit.Points} / ${crit.PointsMax}` : `${Math.round(scoreRatio * 100)}%`
                        }
                      } else {
                        const rep = res.reponses[col.id]
                        if (rep) {
                          if (col.type === 'qcm') {
                            const isCorrect = rep.correct === true
                            if (showResults) {
                              cellClass = isCorrect ? 'cell-correct' : 'cell-incorrect'
                            }
                            const prefix = isCorrect ? '✓ ' : '✗ '
                            const choiceLetter = String.fromCharCode(65 + (rep.choix ?? 0))
                            cellContent = showResponses ? `${prefix}${choiceLetter}` : (isCorrect ? '✓' : '✗')
                          } else {
                            const hasResponse = rep.texte && rep.texte.trim()
                            if (showResults) {
                              cellClass = hasResponse ? 'cell-answered' : 'cell-unanswered'
                            }
                            cellContent = showResponses && hasResponse
                              ? (rep.texte.length > 30 ? `${rep.texte.substring(0, 30)}...` : rep.texte)
                              : (hasResponse ? 'Rempli' : 'Vide')
                            }
                          }
                        }

                      return (
                        <td
                          key={col.id}
                          className={`td-response clickable ${cellClass}`}
                          onClick={() => navigate(`/sessions/${res.session_id}`)}
                        >
                          {cellContent}
                        </td>
                      )
                    })}
                  </tr>
                )
              })}

              {/* Ligne des totaux de classe */}
              {resultats.length > 0 && (
                <tr className="socrative-totals-row">
                  <td className="td-student-name"><strong>Total de classe</strong></td>
                  <td className="td-student-score">
                    <strong>
                      {Math.round(
                        resultats.reduce((acc, r) => {
                          const val = r.note_evaluation !== null ? r.note_evaluation : r.score_auto
                          return acc + (val || 0)
                        }, 0) / resultats.length / 20 * 100
                      )}%
                    </strong>
                  </td>
                  {columns.map((col) => (
                    <td key={col.id} className="td-response-total">
                      <strong>{getColumnStats(col.id)}</strong>
                    </td>
                  ))}
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
