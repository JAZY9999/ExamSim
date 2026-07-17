// Administration des classes — réservé au rôle admin.
// Permet de créer des classes, d'y ajouter/retirer des étudiants,
// et de les supprimer. Utilise les mêmes endpoints que le ciblage d'examens.
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import './PageCommon.css'
import './AdminClassesPage.css'

export default function AdminClassesPage() {
  const [classes, setClasses] = useState([])
  const [etudiants, setEtudiants] = useState([])   // tous les étudiants (pour l'ajout)
  const [nomClasse, setNomClasse] = useState('')
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')
  const [busy, setBusy] = useState(false)
  const [expanded, setExpanded] = useState(null)  // id de la classe ouverte
  const [addingTo, setAddingTo] = useState(null)  // id de la classe en cours d'ajout d'étudiant
  const [searchEtudiant, setSearchEtudiant] = useState('')

  function load() {
    api('/classes').then(setClasses).catch((e) => setError(e.message))
    api('/etudiants').then(setEtudiants).catch(() => {})
  }
  useEffect(load, [])

  function flash(msg) {
    setInfo(msg)
    setError('')
    setTimeout(() => setInfo(''), 4000)
  }

  // Créer une classe
  async function creerClasse(e) {
    e.preventDefault()
    if (!nomClasse.trim()) return
    setBusy(true)
    try {
      await api('/admin/classes', { method: 'POST', body: { nom: nomClasse.trim() } })
      setNomClasse('')
      flash(`Classe « ${nomClasse.trim()} » créée.`)
      load()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  // Supprimer une classe
  async function supprimerClasse(c) {
    if (!window.confirm(`Supprimer la classe « ${c.nom} » ?\n\nLes étudiants ne seront pas supprimés, mais l'assignation des examens liée à cette classe sera retirée.`)) return
    try {
      await api(`/admin/classes/${c.id}`, { method: 'DELETE' })
      flash(`Classe « ${c.nom} » supprimée.`)
      if (expanded === c.id) setExpanded(null)
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  // Ajouter un étudiant à une classe
  async function ajouterMembre(classeId, etudiantId) {
    try {
      await api(`/admin/classes/${classeId}/membres`, { method: 'POST', body: { utilisateur_id: etudiantId } })
      flash('Étudiant ajouté à la classe.')
      setAddingTo(null)
      setSearchEtudiant('')
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  // Retirer un étudiant d'une classe
  async function retirerMembre(classeId, etudiantId, nom) {
    if (!window.confirm(`Retirer ${nom} de la classe ?`)) return
    try {
      await api(`/admin/classes/${classeId}/membres/${etudiantId}`, { method: 'DELETE' })
      flash(`${nom} retiré(e) de la classe.`)
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  // Étudiants non encore membres de la classe sélectionnée
  function etudiantsDisponibles(classe) {
    const membreIds = new Set(classe.membres.map((m) => m.id))
    return etudiants.filter((e) => !membreIds.has(e.id))
  }

  return (
    <div>
      <div className="page-hero">
        <h1>Gestion des classes</h1>
        <p className="muted">
          Créez des classes, affectez-y des étudiants et associez-les à des examens lors de la création.
        </p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {info && <div className="alert alert-success">{info}</div>}

      {/* Créer une classe */}
      <div className="card classes-create-bar">
        <form onSubmit={creerClasse} className="classes-create-form">
          <input
            className="input"
            placeholder="Nom de la nouvelle classe (ex : B2 Informatique)"
            value={nomClasse}
            onChange={(e) => setNomClasse(e.target.value)}
          />
          <button className="btn btn-primary" disabled={busy || !nomClasse.trim()}>
            {busy ? '...' : <><i className="bi bi-plus-lg" /> Créer la classe</>}
          </button>
        </form>
      </div>

      {/* Liste des classes */}
      <h2 className="section-title">Classes ({classes.length})</h2>

      {classes.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: '40px 20px' }}>
          <p className="muted" style={{ fontSize: 16 }}>
            Aucune classe pour le moment.<br />
            Créez votre première classe ci-dessus.
          </p>
        </div>
      ) : (
        <div className="classes-list">
          {classes.map((c) => {
            const isOpen = expanded === c.id
            const isAdding = addingTo === c.id
            const disponibles = etudiantsDisponibles(c).filter((e) =>
              searchEtudiant.trim() === '' ||
              `${e.prenom} ${e.nom} ${e.email}`.toLowerCase().includes(searchEtudiant.toLowerCase())
            )

            return (
              <div key={c.id} className={`classe-card card${isOpen ? ' open' : ''}`}>
                {/* En-tête de la classe */}
                <div className="classe-header" onClick={() => setExpanded(isOpen ? null : c.id)}>
                  <div className="classe-info">
                    <i className="classe-icon bi bi-building" />
                    <div>
                      <strong className="classe-nom">{c.nom}</strong>
                      <span className="muted classe-count">
                        {c.membres?.length ?? 0} étudiant{c.membres?.length !== 1 ? 's' : ''}
                      </span>
                    </div>
                  </div>
                  <div className="classe-actions">
                    <button
                      type="button"
                      className="btn btn-ghost btn-small btn-delete"
                      title="Supprimer la classe"
                      onClick={(e) => { e.stopPropagation(); supprimerClasse(c) }}
                    >
                      <i className="bi bi-trash" /> Supprimer
                    </button>
                    <i className={`chevron bi ${isOpen ? 'bi-chevron-up' : 'bi-chevron-down'}`} />
                  </div>
                </div>

                {/* Corps : liste des membres + ajout */}
                {isOpen && (
                  <div className="classe-body">
                    {/* Membres existants */}
                    {c.membres && c.membres.length > 0 ? (
                      <table className="table membres-table">
                        <thead>
                          <tr>
                            <th>Étudiant</th>
                            <th>Email</th>
                            <th></th>
                          </tr>
                        </thead>
                        <tbody>
                          {c.membres.map((m) => (
                            <tr key={m.id}>
                              <td><strong>{m.prenom} {m.nom}</strong></td>
                              <td>{m.email}</td>
                              <td>
                                <button
                                  type="button"
                                  className="btn btn-ghost btn-small btn-delete"
                                  onClick={() => retirerMembre(c.id, m.id, `${m.prenom} ${m.nom}`)}
                                >
                                  <i className="bi bi-x-lg" /> Retirer
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    ) : (
                      <p className="muted" style={{ marginBottom: 16 }}>
                        Aucun étudiant dans cette classe.
                      </p>
                    )}

                    {/* Bouton / formulaire d'ajout */}
                    {!isAdding ? (
                      <button
                        type="button"
                        className="btn btn-ghost"
                        onClick={() => { setAddingTo(c.id); setSearchEtudiant('') }}
                      >
                        <i className="bi bi-plus-lg" /> Ajouter un étudiant
                      </button>
                    ) : (
                      <div className="add-membre-panel">
                        <input
                          className="input"
                          placeholder="Rechercher un étudiant par nom ou email..."
                          value={searchEtudiant}
                          onChange={(e) => setSearchEtudiant(e.target.value)}
                          autoFocus
                        />
                        <div className="add-membre-list">
                          {disponibles.length === 0 ? (
                            <p className="muted" style={{ padding: 12 }}>
                              {searchEtudiant ? 'Aucun résultat.' : 'Tous les étudiants sont déjà membres.'}
                            </p>
                          ) : (
                            disponibles.slice(0, 10).map((e) => (
                              <div
                                key={e.id}
                                className="add-membre-item"
                                onClick={() => ajouterMembre(c.id, e.id)}
                              >
                                <span><strong>{e.prenom} {e.nom}</strong></span>
                                <span className="muted">{e.email}</span>
                              </div>
                            ))
                          )}
                        </div>
                        <button
                          type="button"
                          className="btn btn-ghost btn-small"
                          onClick={() => { setAddingTo(null); setSearchEtudiant('') }}
                        >
                          Annuler
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
