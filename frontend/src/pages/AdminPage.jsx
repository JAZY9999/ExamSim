// Administration : gestion complète des comptes (réservé au rôle admin).
// User Story : « En tant qu'administrateur, je veux créer des comptes
// (profs/élèves), définir leur statut, réinitialiser les mots de passe... »
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import './PageCommon.css'
import './AdminPage.css'

const ROLES = [
  { value: 'etudiant', label: 'Étudiant' },
  { value: 'examinateur', label: 'Examinateur' },
  { value: 'admin', label: 'Administrateur' },
]

const emptyForm = { prenom: '', nom: '', email: '', mot_de_passe: '', role: 'etudiant' }

export default function AdminPage() {
  const { user: me } = useAuth()
  const [users, setUsers] = useState([])
  const [form, setForm] = useState(emptyForm)
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')
  const [busy, setBusy] = useState(false)

  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value })

  function load() {
    api('/admin/utilisateurs').then(setUsers).catch((e) => setError(e.message))
  }
  useEffect(load, [])

  function flash(msg) {
    setInfo(msg)
    setError('')
    setTimeout(() => setInfo(''), 4000)
  }

  async function creerCompte(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const u = await api('/admin/utilisateurs', { method: 'POST', body: form })
      setForm(emptyForm)
      flash(`Compte créé : ${u.email} (${u.role})`)
      load()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function changerRole(u, role) {
    setError('')
    try {
      await api(`/admin/utilisateurs/${u.id}/role`, { method: 'PATCH', body: { role } })
      flash(`Rôle de ${u.email} → ${role}`)
      load()
    } catch (err) {
      setError(err.message)
      load() // recharge pour annuler visuellement le changement refusé
    }
  }

  async function resetPassword(u) {
    const mdp = window.prompt(`Nouveau mot de passe pour ${u.email} (min. 8 caractères) :`)
    if (!mdp) return
    setError('')
    try {
      await api(`/admin/utilisateurs/${u.id}/password`, { method: 'POST', body: { mot_de_passe: mdp } })
      flash(`Mot de passe de ${u.email} réinitialisé`)
    } catch (err) {
      setError(err.message)
    }
  }

  async function supprimer(u) {
    if (!window.confirm(`Supprimer définitivement le compte de ${u.prenom} ${u.nom} (${u.email}) ?\n\nSes sessions, réponses et évaluations seront également supprimées.`)) return
    setError('')
    try {
      await api(`/admin/utilisateurs/${u.id}`, { method: 'DELETE' })
      flash(`Compte ${u.email} supprimé`)
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <div>
      <div className="page-hero">
        <h1>Administration des comptes</h1>
        <p className="muted">
          Créez les comptes, définissez les rôles et réinitialisez les mots de passe.
        </p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {info && <div className="alert alert-success">{info}</div>}

      {/* --- Création de compte --- */}
      <div className="card admin-create">
        <h2 className="section-title">Créer un compte</h2>
        <form onSubmit={creerCompte} className="admin-create-form">
          <input className="input" placeholder="Prénom" value={form.prenom} onChange={set('prenom')} />
          <input className="input" placeholder="Nom *" value={form.nom} onChange={set('nom')} required />
          <input className="input" type="email" placeholder="Email *" value={form.email} onChange={set('email')} required />
          <input className="input" type="password" placeholder="Mot de passe * (min. 8)"
            value={form.mot_de_passe} onChange={set('mot_de_passe')} required minLength={8} />
          <select className="select" value={form.role} onChange={set('role')}>
            {ROLES.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
          </select>
          <button className="btn btn-primary" disabled={busy}>
            {busy ? '...' : '+ Créer'}
          </button>
        </form>
      </div>

      {/* --- Liste des comptes --- */}
      <h2 className="section-title">Comptes ({users.length})</h2>
      <table className="table">
        <thead>
          <tr><th>Nom</th><th>Email</th><th>Rôle</th><th>Créé le</th><th>Actions</th></tr>
        </thead>
        <tbody>
          {users.map((u) => {
            const isMe = u.id === me?.id
            return (
              <tr key={u.id}>
                <td>
                  <strong>{u.prenom} {u.nom}</strong>
                  {isMe && <span className="tag" style={{ marginLeft: 8 }}>vous</span>}
                </td>
                <td>{u.email}</td>
                <td>
                  {/* Changement de rôle direct — désactivé sur son propre compte */}
                  <select className="select select-role" value={u.role} disabled={isMe}
                    onChange={(e) => changerRole(u, e.target.value)}>
                    {ROLES.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
                  </select>
                </td>
                <td>{new Date(u.created_at).toLocaleDateString('fr-FR')}</td>
                <td>
                  <div className="admin-actions">
                    <button className="btn btn-ghost btn-small" title="Réinitialiser le mot de passe"
                      onClick={() => resetPassword(u)}>🔑 Reset MDP</button>
                    <button className="btn btn-ghost btn-small btn-delete" title="Supprimer le compte"
                      disabled={isMe} onClick={() => supprimer(u)}>🗑 Supprimer</button>
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
