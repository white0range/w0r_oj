function decodeBase64Url(input) {
  const normalized = input.replace(/-/g, '+').replace(/_/g, '/')
  const padding = normalized.length % 4
  const padded = padding ? normalized + '='.repeat(4 - padding) : normalized
  return atob(padded)
}

export function parseToken(token) {
  if (!token) {
    return {}
  }

  try {
    const [, payload] = token.split('.')
    const decoded = JSON.parse(decodeBase64Url(payload))
    return {
      userId: Number(decoded.user_id || 0),
      username: decoded.username || '',
      role: Number(decoded.role || 0),
    }
  } catch {
    return {}
  }
}

export function readSession() {
  localStorage.removeItem('token')

  return {
    token: '',
    userId: Number(localStorage.getItem('user_id') || 0),
    username: localStorage.getItem('username') || '',
    role: Number(localStorage.getItem('role') || 0),
  }
}

export function persistSession({ userId, username, role }) {
  localStorage.setItem('user_id', String(userId || 0))
  localStorage.setItem('username', username || '')
  localStorage.setItem('role', String(role || 0))
}

export function clearSession() {
  localStorage.removeItem('token')
  localStorage.removeItem('user_id')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
}
