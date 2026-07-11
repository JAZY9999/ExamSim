// Page 2 (zoning) : Sidebar de navigation à gauche, Header en haut,
// zone de contenu centrale (rendue via <Outlet />).
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import './DashboardLayout.css'

const NAV = [
  { to: '/', label: 'Accueil', icon: '🏠', end: true },
  { to: '/entrainements', label: 'Mes entraînements', icon: '📝' },
  { to: '/examens', label: 'Examens officiels', icon: '🎯' },
  // La création et la gestion de sujets sont réservées au personnel.
  { to: '/creer', label: 'Créer un sujet', icon: '➕', roles: ['examinateur', 'admin'] },
  { to: '/mes-sujets', label: 'Mes sujets', icon: '📋', roles: ['examinateur', 'admin'] },
  { to: '/statistiques', label: 'Statistiques', icon: '📊' },
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
        <div className="sidebar-logo">🎓 <span>ExamSim</span></div>
        <nav className="sidebar-nav">
          {NAV.filter((item) => !item.roles || item.roles.includes(user?.role)).map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end}
              className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
              <span className="nav-icon">{item.icon}</span>{item.label}
            </NavLink>
          ))}
          {user?.role === 'admin' && (
            <>
              <NavLink to="/admin" end
                className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                <span className="nav-icon">⚙️</span>Administration
              </NavLink>
              <NavLink to="/admin/classes"
                className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                <span className="nav-icon">🏫</span>Classes
              </NavLink>
              <NavLink to="/admin/journal"
                className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                <span className="nav-icon">📜</span>Journal d'activité
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
            <span className="search-icon">🔍</span>
            <input className="search-input" placeholder="Rechercher un sujet, un examen..." />
          </div>
          <div className="header-right">
            <button className="icon-btn" title="Notifications">🔔</button>
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
