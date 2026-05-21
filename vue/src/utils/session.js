export function parseToken(token) {
  if (!token) {
    return {}
  }

  try {
    const [, payload] = token.split('.')
    const decoded = JSON.parse(atob(payload))
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
  const token = localStorage.getItem('token') || ''
  const parsed = parseToken(token)

  return {
    token,
    userId: Number(localStorage.getItem('user_id') || parsed.userId || 0),
    username: localStorage.getItem('username') || parsed.username || '',
    role: Number(localStorage.getItem('role') || parsed.role || 0),
  }
}

export function persistSession({ token, userId, username, role }) {
  localStorage.setItem('token', token || '')
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
