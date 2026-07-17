// Page 1 : Authentification (Connexion / Inscription)
// Zoning : écran scindé en deux — illustration à gauche, formulaire à droite.
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import './AuthPage.css'

export default function AuthPage() {
  const { login, register } = useAuth()
  const navigate = useNavigate()

  const [mode, setMode] = useState('login') // 'login' | 'register'
  const [form, setForm] = useState({
    nom: '', prenom: '', email: '', mot_de_passe: '', role: 'etudiant',
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value })

  async function submit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (mode === 'login') {
        await login(form.email, form.mot_de_passe)
      } else {
        await register(form)
      }
      navigate('/')
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth">
      {/* Colonne gauche : illustration + accroche */}
      <div className="auth-hero">
        <div className="auth-logo"><i className="bi bi-mortarboard-fill" /> ExamSim</div>
        <h1>Entraînez-vous.<br />Chronométrez.<br />Évaluez-vous.</h1>
        <p>
          La plateforme collaborative pour préparer vos examens écrits et oraux
          dans les conditions réelles.
        </p>
        <div className="auth-illus" aria-hidden="true">
          <i className="bi bi-mortarboard-fill" />
          <i className="bi bi-easel2-fill" />
          <i className="bi bi-person-video3" />
        </div>
      </div>

      {/* Colonne droite : formulaire centré */}
      <div className="auth-form-wrap">
        <form className="auth-form" onSubmit={submit}>
          <h2>{mode === 'login' ? 'Se connecter' : 'Créer un compte'}</h2>
          <p className="muted" style={{ marginBottom: 20 }}>
            {mode === 'login'
              ? 'Accédez à votre espace d\'entraînement.'
              : 'Rejoignez la plateforme en quelques secondes.'}
          </p>

          {error && <div className="alert alert-error">{error}</div>}

          {mode === 'register' && (
            <div style={{ display: 'flex', gap: 12 }}>
              <div className="field" style={{ flex: 1 }}>
                <label>Prénom</label>
                <input className="input" value={form.prenom} onChange={set('prenom')} />
              </div>
              <div className="field" style={{ flex: 1 }}>
                <label>Nom</label>
                <input className="input" value={form.nom} onChange={set('nom')} />
              </div>
            </div>
          )}

          <div className="field">
            <label>Email</label>
            <input className="input" type="email" value={form.email}
              onChange={set('email')} placeholder="vous@exemple.fr" required />
          </div>

          <div className="field">
            <label>Mot de passe</label>
            <input className="input" type="password" value={form.mot_de_passe}
              onChange={set('mot_de_passe')} placeholder="••••••••" required />
          </div>

          {/* Le rôle n'est choisi qu'à l'inscription : à la connexion, il est
              détecté automatiquement et l'espace s'adapte (étudiant/examinateur/admin). */}
          {mode === 'register' && (
            <div className="field">
              <label>Rôle</label>
              <select className="select" value={form.role} onChange={set('role')}>
                <option value="etudiant">Étudiant</option>
                <option value="examinateur">Examinateur</option>
                <option value="admin">Administrateur</option>
              </select>
            </div>
          )}

          <button className="btn btn-primary btn-block" disabled={busy}>
            {busy ? '...' : mode === 'login' ? 'Se connecter' : 'S\'inscrire'}
          </button>

          <div className="switch-mode">
            {mode === 'login' ? (
              <>Pas encore de compte ?{' '}
                <a href="#" onClick={(e) => { e.preventDefault(); setMode('register'); setError('') }}>
                  Créer un compte
                </a></>
            ) : (
              <>Déjà inscrit ?{' '}
                <a href="#" onClick={(e) => { e.preventDefault(); setMode('login'); setError('') }}>
                  Se connecter
                </a></>
            )}
          </div>

          <div className="demo-hint">
            <strong>Comptes de démo</strong> (mot de passe : <code>password</code>)<br />
            etudiant@examsim.fr · prof@examsim.fr · admin@examsim.fr
          </div>
        </form>
      </div>
    </div>
  )
}
