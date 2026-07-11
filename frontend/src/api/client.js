// Client HTTP minimal pour dialoguer avec le backend Go.
// Le token JWT est stocké dans le localStorage et ajouté à chaque requête.

const TOKEN_KEY = 'examsim_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

// api effectue un appel fetch vers /api/... en injectant le token.
export async function api(path, { method = 'GET', body } = {}) {
  const headers = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`/api${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 204) return null
  const data = await res.json().catch(() => null)

  if (!res.ok) {
    const msg = data?.error || `Erreur ${res.status}`
    throw new Error(msg)
  }
  return data
}
