// Carte d'un examen (utilisée dans la grille du dashboard).
import { useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import ExamCompletionModal from './ExamCompletionModal'
import './ExamCard.css'

const MODALITE_LABEL = { qcm: 'QCM', cas_pratique: 'Cas pratique', oral: 'Oral' }

function fmtDateTime(iso) {
  if (!iso) return null
  return new Date(iso).toLocaleString('fr-FR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

export default function ExamCard({ examen, onError }) {
  const navigate = useNavigate()
  const { user } = useAuth()
  const isEtudiant = user?.role === 'etudiant'
  const isStaff = !isEtudiant

  const [showModal, setShowModal] = useState(false)

  const a = examen.assignations
  const isCible = a && (a.classes?.length > 0 || a.etudiants?.length > 0)
  const resume = examen.sessions_resume

  async function demarrer() {
    try {
      const session = await api('/sessions', {
        method: 'POST',
        body: { examen_id: examen.id },
      })
      if (examen.modalite === 'oral') {
        navigate(`/oral/${session.id}`)
      } else {
        navigate(`/examen/${session.id}/passer`)
      }
    } catch (err) {
      onError?.(err.message)
    }
  }

  return (
    <>
      <div className={`exam-card${isCible ? ' exam-card--cible' : ''}${resume?.tous_termines ? ' exam-card--done' : ''}`}>
        {/* Ligne supérieure */}
        <div className="exam-card-top">
          <span className={`mod-pill mod-${examen.modalite}`}>
            {MODALITE_LABEL[examen.modalite] || examen.modalite}
          </span>
          {examen.type === 'officiel' && <span className="tag tag-warning">Officiel</span>}

          {/* Badges staff */}
          {isStaff && isCible && (
            <span className="exam-badge-cible" data-tooltip={buildTooltipText(a)}>
              🎯 Ciblé
              <span className="badge-tooltip">{buildTooltipNode(a)}</span>
            </span>
          )}
          {isStaff && !isCible && <span className="exam-badge-public">🌐 Public</span>}

          {/* Badge sessions résumé */}
          {isStaff && resume && (
            <button
              className={`sessions-badge${resume.tous_termines ? ' sessions-badge--done' : ''}`}
              onClick={() => resume.tous_termines && setShowModal(true)}
              title={resume.tous_termines ? 'Tous les étudiants ont terminé — cliquez pour vérifier' : `${resume.terminees}/${resume.total} ont terminé`}
            >
              {resume.tous_termines ? '✅ Tous terminés' : `${resume.terminees}/${resume.total}`}
            </button>
          )}
        </div>

        <h3 className="exam-card-title">{examen.titre}</h3>
        <p className="exam-card-desc">{examen.description}</p>

        <div className="exam-card-tags">
          {(examen.tags || []).map((t) => <span key={t} className="tag">{t}</span>)}
        </div>

        {/* Fenêtre de disponibilité */}
        {(examen.disponible_de || examen.disponible_jusqu_a) && (
          <div className="exam-dispo">
            {examen.disponible_de && <span>📅 Dès le {fmtDateTime(examen.disponible_de)}</span>}
            {examen.disponible_jusqu_a && <span>🔒 Jusqu&apos;au {fmtDateTime(examen.disponible_jusqu_a)}</span>}
          </div>
        )}

        <div className="exam-card-footer">
          {/* Créateur — visible par les étudiants */}
          {isEtudiant && examen.createur_nom && (
            <span className="exam-creator">👨‍🏫 {examen.createur_nom}</span>
          )}
          <span className="muted">⏱ {examen.duree_min} min</span>
          {isEtudiant ? (
            <button className="btn btn-primary" onClick={demarrer}>
              {examen.modalite === 'oral' ? 'Passer l\'oral' : 'Démarrer'}
            </button>
          ) : (
            <span className="muted" style={{ fontSize: 13 }}>
              {examen.modalite === 'oral'
                ? 'Rejoignez depuis « Oraux en cours »'
                : 'Réservé aux étudiants'}
            </span>
          )}
        </div>
      </div>

      {/* Modal de complétion — cliquable sur le badge "Tous terminés" */}
      {showModal && (
        <ExamCompletionModal
          examen={examen}
          onClose={() => setShowModal(false)}
        />
      )}
    </>
  )
}

function buildTooltipText(a) {
  const parts = []
  if (a.classes?.length > 0) parts.push(`Classes : ${a.classes.map((c) => c.nom).join(', ')}`)
  if (a.etudiants?.length > 0) parts.push(`Étudiants : ${a.etudiants.map((e) => e.nom).join(', ')}`)
  return parts.join(' | ')
}

function buildTooltipNode(a) {
  return (
    <>
      {a.classes?.length > 0 && (
        <div className="tooltip-section">
          <strong>🏫 Classes</strong>
          <ul>{a.classes.map((c) => <li key={c.id}>{c.nom}</li>)}</ul>
        </div>
      )}
      {a.etudiants?.length > 0 && (
        <div className="tooltip-section">
          <strong>👤 Étudiants</strong>
          <ul>{a.etudiants.map((e) => <li key={e.id}>{e.nom}</li>)}</ul>
        </div>
      )}
    </>
  )
}
