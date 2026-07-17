// Accueil du dashboard — le contenu s'adapte au rôle détecté à la connexion :
//  - Étudiant : grille de sujets à démarrer.
//  - Examinateur/Admin : oraux EN COURS à rejoindre (même session que
//    l'étudiant → même timer, même salle de visio), puis la liste des sujets
//    avec une barre de filtres (modalité, type, ciblage, classe, recherche).
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import ExamCard from '../components/ExamCard'
import './PageCommon.css'
import './HomePage.css'

export default function HomePage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const isExaminateur = user?.role === 'examinateur' || user?.role === 'admin'

  const [examens, setExamens] = useState([])
  const [classes, setClasses] = useState([])
  const [activeOrals, setActiveOrals] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // --- Filtres (staff uniquement) ---
  const [filterRecherche, setFilterRecherche] = useState('')
  const [filterModalite, setFilterModalite] = useState('toutes')
  const [filterType, setFilterType] = useState('tous')
  const [filterCiblage, setFilterCiblage] = useState('tous') // 'tous' | 'public' | 'cible'
  const [filterClasse, setFilterClasse] = useState('toutes')

  useEffect(() => {
    api('/examens')
      .then(setExamens)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  // Chargement des classes pour le filtre (staff seulement)
  useEffect(() => {
    if (!isExaminateur) return
    api('/classes').then(setClasses).catch(() => {})
  }, [isExaminateur])

  // Côté examinateur : on rafraîchit la liste des oraux en cours toutes les 5 s
  useEffect(() => {
    if (!isExaminateur) return
    const load = () => api('/sessions/active-orals').then(setActiveOrals).catch(() => {})
    load()
    const t = setInterval(load, 5000)
    return () => clearInterval(t)
  }, [isExaminateur])

  // --- Application des filtres ---
  const examensFiltres = useMemo(() => {
    if (!isExaminateur) return examens
    return examens.filter((e) => {
      // Recherche textuelle
      if (filterRecherche.trim()) {
        const q = filterRecherche.toLowerCase()
        const inTitre = e.titre.toLowerCase().includes(q)
        const inDesc  = e.description?.toLowerCase().includes(q)
        const inTags  = e.tags?.some((t) => t.toLowerCase().includes(q))
        if (!inTitre && !inDesc && !inTags) return false
      }
      // Modalité
      if (filterModalite !== 'toutes' && e.modalite !== filterModalite) return false
      // Type
      if (filterType !== 'tous' && e.type !== filterType) return false
      // Ciblage
      const isCible = e.assignations && (e.assignations.classes?.length > 0 || e.assignations.etudiants?.length > 0)
      if (filterCiblage === 'public' && isCible) return false
      if (filterCiblage === 'cible' && !isCible) return false
      // Classe spécifique
      if (filterClasse !== 'toutes') {
        if (!e.assignations?.classes?.some((c) => c.id === filterClasse)) return false
      }
      return true
    })
  }, [examens, filterRecherche, filterModalite, filterType, filterCiblage, filterClasse, isExaminateur])

  const fmtHeure = (d) => new Date(d).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })

  const nbFiltresActifs = [
    filterRecherche.trim() !== '',
    filterModalite !== 'toutes',
    filterType !== 'tous',
    filterCiblage !== 'tous',
    filterClasse !== 'toutes',
  ].filter(Boolean).length

  function resetFiltres() {
    setFilterRecherche('')
    setFilterModalite('toutes')
    setFilterType('tous')
    setFilterCiblage('tous')
    setFilterClasse('toutes')
  }

  return (
    <div>
      <div className="page-hero" style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1>Bonjour {user?.prenom}</h1>
          <p className="muted">
            {isExaminateur
              ? 'Espace examinateur : rejoignez un oral en cours ou consultez les sujets.'
              : 'Prêt à vous entraîner ? Choisissez un sujet ci-dessous pour démarrer une session.'}
          </p>
        </div>
        {isExaminateur && (
          <button className="btn btn-primary" onClick={() => navigate('/creer')}>
            <i className="bi bi-plus-lg" /> Créer un examen
          </button>
        )}
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {/* --- Oraux en direct (examinateur) --- */}
      {isExaminateur && (
        <>
          <h2 className="section-title">Oraux en cours</h2>
          {activeOrals.length > 0 ? (
            <div className="card-grid" style={{ marginBottom: 32 }}>
              {activeOrals.map((s) => (
                <div key={s.id} className="card oral-live-card">
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span className="tag tag-danger"><i className="bi bi-record-circle-fill" /> En direct</span>
                    <span className="muted" style={{ fontSize: 13 }}>depuis {fmtHeure(s.debut_at)}</span>
                  </div>
                  <h3 style={{ margin: '10px 0 4px' }}>{s.examen_titre}</h3>
                  <p className="muted" style={{ marginBottom: 14 }}>
                    Candidat : <strong>{s.etudiant}</strong> · {s.duree_min} min
                  </p>
                  <button className="btn btn-primary" onClick={() => navigate(`/oral/${s.id}`)}>
                    Rejoindre en tant qu&apos;examinateur
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <p className="muted" style={{ marginBottom: 32 }}>
              Aucun oral en cours pour le moment. Dès qu&apos;un étudiant démarre une
              épreuve orale, elle apparaîtra ici automatiquement.
            </p>
          )}
        </>
      )}

      {/* --- Titre + barre de filtres --- */}
      <div className="section-header">
        <h2 className="section-title" style={{ margin: 0 }}>
          Sujets disponibles
          {isExaminateur && (
            <span className="section-count">
              {examensFiltres.length} / {examens.length}
            </span>
          )}
        </h2>

        {/* Filtres — staff uniquement */}
        {isExaminateur && (
          <div className="filters-bar">
            {/* Recherche */}
            <div className="filter-search">
              <i className="filter-search-icon bi bi-search" />
              <input
                className="input filter-input"
                placeholder="Rechercher un sujet..."
                value={filterRecherche}
                onChange={(e) => setFilterRecherche(e.target.value)}
              />
            </div>

            {/* Modalité */}
            <select className="select filter-select" value={filterModalite}
              onChange={(e) => setFilterModalite(e.target.value)}>
              <option value="toutes">Toutes modalités</option>
              <option value="qcm">QCM</option>
              <option value="cas_pratique">Cas pratique</option>
              <option value="oral">Oral</option>
            </select>

            {/* Type */}
            <select className="select filter-select" value={filterType}
              onChange={(e) => setFilterType(e.target.value)}>
              <option value="tous">Tous types</option>
              <option value="entrainement">Entraînement</option>
              <option value="officiel">Officiel</option>
            </select>

            {/* Ciblage */}
            <select className="select filter-select" value={filterCiblage}
              onChange={(e) => { setFilterCiblage(e.target.value); if (e.target.value !== 'cible') setFilterClasse('toutes') }}>
              <option value="tous">Tout ciblage</option>
              <option value="public">Public</option>
              <option value="cible">Ciblé</option>
            </select>

            {/* Classe (visible seulement si filtre ciblage = ciblé) */}
            {filterCiblage === 'cible' && classes.length > 0 && (
              <select className="select filter-select" value={filterClasse}
                onChange={(e) => setFilterClasse(e.target.value)}>
                <option value="toutes">Toutes classes</option>
                {classes.map((c) => (
                  <option key={c.id} value={c.id}>{c.nom}</option>
                ))}
              </select>
            )}

            {/* Reset */}
            {nbFiltresActifs > 0 && (
              <button className="btn btn-ghost btn-small" onClick={resetFiltres}>
                <i className="bi bi-x-lg" /> Réinitialiser ({nbFiltresActifs})
              </button>
            )}
          </div>
        )}
      </div>

      {/* --- Grille des examens --- */}
      {loading ? (
        <div className="center" style={{ padding: 60 }}><div className="spinner" /></div>
      ) : (
        <div className="card-grid">
          {examensFiltres.map((e) => <ExamCard key={e.id} examen={e} onError={setError} />)}
          {examensFiltres.length === 0 && (
            <div className="empty-state">
              <i className="bi bi-inboxes empty-state-icon" />
              <p>Aucun examen ne correspond aux filtres.</p>
              {nbFiltresActifs > 0 && (
                <button className="btn btn-ghost btn-small" onClick={resetFiltres}>
                  Voir tous les sujets
                </button>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
