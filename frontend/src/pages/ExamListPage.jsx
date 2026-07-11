// Liste filtrée des examens (entraînements ou officiels).
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import ExamCard from '../components/ExamCard'
import './PageCommon.css'

export default function ExamListPage({ filter }) {
  const [examens, setExamens] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    api('/examens')
      .then((data) => setExamens(data.filter((e) => e.type === filter)))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [filter])

  const titre = filter === 'officiel' ? 'Examens officiels' : 'Mes entraînements'

  return (
    <div>
      <div className="page-hero">
        <h1>{titre}</h1>
        <p className="muted">
          {filter === 'officiel'
            ? 'Épreuves encadrées par un examinateur (écrites ou orales).'
            : 'Sujets d\'entraînement pour réviser dans les conditions réelles.'}
        </p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="center" style={{ padding: 60 }}><div className="spinner" /></div>
      ) : (
        <div className="card-grid">
          {examens.map((e) => <ExamCard key={e.id} examen={e} onError={setError} />)}
          {examens.length === 0 && <p className="muted">Aucun examen dans cette catégorie.</p>}
        </div>
      )}
    </div>
  )
}
