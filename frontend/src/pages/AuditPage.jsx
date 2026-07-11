// Journal d'activité (admin) : consultation du journal d'audit.
// Chaque action sensible de la plateforme y est tracée — y compris les
// tentatives de connexion échouées et les actions sur des comptes supprimés.
import { useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import './PageCommon.css'
import './AuditPage.css'

// Libellés et couleurs par type d'action.
const ACTIONS = {
  connexion:               { label: 'Connexion',            cls: 'audit-ok' },
  connexion_echec:         { label: 'Échec connexion',      cls: 'audit-danger' },
  inscription:             { label: 'Inscription',          cls: 'audit-ok' },
  examen_cree:             { label: 'Examen créé',          cls: 'audit-info' },
  session_demarree:        { label: 'Session démarrée',     cls: 'audit-info' },
  session_soumise:         { label: 'Session soumise',      cls: 'audit-info' },
  evaluation_enregistree:  { label: 'Évaluation',           cls: 'audit-ok' },
  compte_cree:             { label: 'Compte créé',          cls: 'audit-admin' },
  role_modifie:            { label: 'Rôle modifié',         cls: 'audit-admin' },
  mdp_reinitialise:        { label: 'MDP réinitialisé',     cls: 'audit-admin' },
  compte_supprime:         { label: 'Compte supprimé',      cls: 'audit-danger' },
}

export default function AuditPage() {
  const [entries, setEntries] = useState([])
  const [error, setError] = useState('')
  const [filtreAction, setFiltreAction] = useState('')
  const [recherche, setRecherche] = useState('')

  useEffect(() => {
    api('/admin/audit').then(setEntries).catch((e) => setError(e.message))
  }, [])

  const filtered = useMemo(() => {
    const q = recherche.toLowerCase()
    return entries.filter((e) =>
      (!filtreAction || e.action === filtreAction) &&
      (!q || `${e.acteur} ${e.acteur_email} ${e.details}`.toLowerCase().includes(q))
    )
  }, [entries, filtreAction, recherche])

  const fmt = (d) => new Date(d).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'medium' })

  return (
    <div>
      <div className="page-hero">
        <h1>Journal d'activité</h1>
        <p className="muted">
          Traçabilité de toutes les actions sensibles ({entries.length} entrées récentes).
          Les traces sont conservées même après suppression d'un compte.
        </p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {/* Filtres */}
      <div className="audit-filters">
        <input className="input" placeholder="🔍 Filtrer par utilisateur, email ou détail..."
          value={recherche} onChange={(e) => setRecherche(e.target.value)} />
        <select className="select" value={filtreAction} onChange={(e) => setFiltreAction(e.target.value)}>
          <option value="">Toutes les actions</option>
          {Object.entries(ACTIONS).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
        </select>
      </div>

      <table className="table">
        <thead>
          <tr><th>Date</th><th>Acteur</th><th>Action</th><th>Détails</th></tr>
        </thead>
        <tbody>
          {filtered.map((e) => {
            const a = ACTIONS[e.action] || { label: e.action, cls: '' }
            return (
              <tr key={e.id}>
                <td className="audit-date">{fmt(e.created_at)}</td>
                <td>
                  <strong>{e.acteur}</strong>
                  {e.acteur_email && <div className="muted audit-email">{e.acteur_email}</div>}
                </td>
                <td><span className={`audit-badge ${a.cls}`}>{a.label}</span></td>
                <td className="muted">{e.details || '—'}</td>
              </tr>
            )
          })}
          {filtered.length === 0 && (
            <tr><td colSpan={4} className="muted">Aucune entrée ne correspond aux filtres.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
