import { useSyncExternalStore } from 'react'
import { clearSession, parseToken, persistSession, readSession } from '../utils/session'

let session = readSession()
const listeners = new Set()

function subscribe(listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function getSession() {
  return session
}

function setSession(next) {
  session = next
  for (const listener of listeners) listener()
}

function setToken(token) {
  const claims = parseToken(token)
  const next = { token, userId: claims.userId || 0, username: claims.username || '', role: claims.role || 0 }
  persistSession(next)
  setSession(next)
}

export const store = {
  get token() { return session.token },
  login: setToken,
  setToken,
  hydrateProfile(profile) {
    if (profile.id !== session.userId) return
    const next = { ...session, username: profile.username || session.username, role: profile.role }
    persistSession(next)
    setSession(next)
  },
  logout() {
    clearSession()
    setSession({ token: '', userId: 0, username: '', role: 0 })
  },
}

export function useSession() {
  return useSyncExternalStore(subscribe, getSession)
}
