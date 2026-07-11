// Routage principal de l'application.
// Les routes du dashboard sont protégées : elles exigent une authentification.
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import AuthPage from './pages/AuthPage'
import DashboardLayout from './components/DashboardLayout'
import HomePage from './pages/HomePage'
import ExamListPage from './pages/ExamListPage'
import StatsPage from './pages/StatsPage'
import ExamRunPage from './pages/ExamRunPage'
import OralPage from './pages/OralPage'
import AdminPage from './pages/AdminPage'
import CreateExamPage from './pages/CreateExamPage'
import MyExamsPage from './pages/MyExamsPage'
import ExamResultsPage from './pages/ExamResultsPage'
import SessionDetailPage from './pages/SessionDetailPage'
import AuditPage from './pages/AuditPage'
import AdminClassesPage from './pages/AdminClassesPage'

// Garde de route : redirige vers /login si non authentifié.
function Protected({ children }) {
  const { user, loading } = useAuth()
  if (loading) return <div className="center" style={{ height: '100vh' }}><div className="spinner" /></div>
  if (!user) return <Navigate to="/login" replace />
  return children
}

// Garde par rôle : réservé au personnel (examinateur/admin) ou à l'admin seul.
// Un étudiant qui tape l'URL à la main est renvoyé à l'accueil.
function RoleProtected({ roles, children }) {
  const { user } = useAuth()
  if (!roles.includes(user?.role)) return <Navigate to="/" replace />
  return children
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<AuthPage />} />

      {/* Pages plein écran (sans layout dashboard) */}
      <Route path="/examen/:id/passer" element={<Protected><ExamRunPage /></Protected>} />
      <Route path="/oral/:id" element={<Protected><OralPage /></Protected>} />

      {/* Espace principal avec sidebar + header */}
      <Route path="/" element={<Protected><DashboardLayout /></Protected>}>
        <Route index element={<HomePage />} />
        <Route path="entrainements" element={<ExamListPage filter="entrainement" />} />
        <Route path="examens" element={<ExamListPage filter="officiel" />} />
        <Route path="creer" element={
          <RoleProtected roles={['examinateur', 'admin']}><CreateExamPage /></RoleProtected>
        } />
        <Route path="examens/:id/modifier" element={
          <RoleProtected roles={['examinateur', 'admin']}><CreateExamPage editMode /></RoleProtected>
        } />
        <Route path="mes-sujets" element={
          <RoleProtected roles={['examinateur', 'admin']}><MyExamsPage /></RoleProtected>
        } />
        <Route path="examens/:id/resultats" element={
          <RoleProtected roles={['examinateur', 'admin']}><ExamResultsPage /></RoleProtected>
        } />
        <Route path="statistiques" element={<StatsPage />} />
        <Route path="sessions/:id" element={<SessionDetailPage />} />
        <Route path="admin" element={
          <RoleProtected roles={['admin']}><AdminPage /></RoleProtected>
        } />
        <Route path="admin/classes" element={
          <RoleProtected roles={['admin']}><AdminClassesPage /></RoleProtected>
        } />
        <Route path="admin/journal" element={
          <RoleProtected roles={['admin']}><AuditPage /></RoleProtected>
        } />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
