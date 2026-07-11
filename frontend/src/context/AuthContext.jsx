// Contexte d'authentification : conserve l'utilisateur connecté et expose
// les actions login / register / logout à toute l'application.
import { createContext, useContext, useEffect, useState } from 'react'
import { api, setToken, getToken } from '../api/client'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  // Au chargement, si un token existe, on récupère le profil.
  useEffect(() => {
    async function bootstrap() {
      if (getToken()) {
        try {
          const me = await api('/me')
          setUser(me)
        } catch {
          setToken(null)
        }
      }
      setLoading(false)
    }
    bootstrap()
  }, [])

  async function login(email, motDePasse) {
    const res = await api('/auth/login', {
      method: 'POST',
      body: { email, mot_de_passe: motDePasse },
    })
    setToken(res.token)
    setUser(res.utilisateur)
    return res.utilisateur
  }

  async function register(payload) {
    const res = await api('/auth/register', { method: 'POST', body: payload })
    setToken(res.token)
    setUser(res.utilisateur)
    return res.utilisateur
  }

  function logout() {
    setToken(null)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
