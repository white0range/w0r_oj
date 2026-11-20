function decodeBase64Url(input) {
  const normalized = input.replace(/-/g, '+').replace(/_/g, '/')
  const padding = normalized.length % 4
  const padded = padding ? normalized + '='.repeat(4 - padding) : normalized
  return new TextDecoder().decode(Uint8Array.from(atob(padded), (char) => char.charCodeAt(0)))
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
      expiresAt: Number(decoded.exp || 0) * 1000,
    }
  } catch {
    return {}
  }
}

export function readSession() {
  try { localStorage.removeItem('token') } catch {}
  return { token: '', userId: 0, username: '', role: 0 }
}

export function persistSession({ userId, username, role }) {
  try {
    localStorage.setItem('user_id', String(userId || 0))
    localStorage.setItem('username', username || '')
    localStorage.setItem('role', String(role || 0))
  } catch {}
}

export function clearSession() {
  try {
    for (const key of ['token', 'user_id', 'username', 'role']) localStorage.removeItem(key)
  } catch {}
}
