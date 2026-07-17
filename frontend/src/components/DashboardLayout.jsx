// Page 2 (zoning) : Sidebar de navigation à gauche, Header en haut,
// zone de contenu centrale (rendue via <Outlet />).
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import './DashboardLayout.css'

const NAV = [
  { to: '/', label: 'Accueil', icon: 'bi-house-door', end: true },
  { to: '/entrainements', label: 'Mes entraînements', icon: 'bi-journal-text' },
  { to: '/examens', label: 'Examens officiels', icon: 'bi-bullseye' },
  // La création et la gestion de sujets sont réservées au personnel.
  { to: '/creer', label: 'Créer un sujet', icon: 'bi-plus-circle', roles: ['examinateur', 'admin'] },
  { to: '/mes-sujets', label: 'Mes sujets', icon: 'bi-list-check', roles: ['examinateur', 'admin'] },
  { to: '/statistiques', label: 'Statistiques', icon: 'bi-bar-chart' },
]

export default function DashboardLayout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const initials = ((user?.prenom?.[0] || '') + (user?.nom?.[0] || '')).toUpperCase() || '?'

  function handleLogout() {
    logout()
    navigate('/login')
  }

  return (
    <div className="layout">
      {/* Sidebar */}
      <aside className="sidebar">
        <div className="sidebar-logo"><i className="bi bi-mortarboard-fill" /> <span>ExamSim</span></div>
        <nav className="sidebar-nav">
          {NAV.filter((item) => !item.roles || item.roles.includes(user?.role)).map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end}
              className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
              <i className={`nav-icon bi ${item.icon}`} />{item.label}
            </NavLink>
          ))}
          {user?.role === 'admin' && (
            <>
              <NavLink to="/admin" end
                className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                <i className="nav-icon bi bi-gear" />Administration
              </NavLink>
              <NavLink to="/admin/classes"
                className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                <i className="nav-icon bi bi-building" />Classes
              </NavLink>
              <NavLink to="/admin/journal"
                className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                <i className="nav-icon bi bi-journal-text" />Journal d'activité
              </NavLink>
            </>
          )}
        </nav>
        <div className="sidebar-footer">
          <span className={`role-badge role-${user?.role}`}>{roleLabel(user?.role)}</span>
        </div>
      </aside>

      {/* Colonne principale */}
      <div className="main">
        {/* Header */}
        <header className="header">
          <div className="search">
            <i className="search-icon bi bi-search" />
            <input className="search-input" placeholder="Rechercher un sujet, un examen..." />
          </div>
          <div className="header-right">
            <button className="icon-btn" title="Notifications"><i className="bi bi-bell" /></button>
            <div className="avatar" title={`${user?.prenom} ${user?.nom}`}>{initials}</div>
            <button className="btn btn-ghost" onClick={handleLogout}>Déconnexion</button>
          </div>
        </header>

        {/* Contenu central */}
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function roleLabel(role) {
  return { etudiant: 'Étudiant', examinateur: 'Examinateur', admin: 'Administrateur' }[role] || role
}
