import { reactive } from 'vue'
import { clearSession, parseToken, persistSession, readSession } from '../utils/session'

const session = readSession()

export const store = reactive({
  token: session.token,
  userId: session.userId,
  username: session.username,
  role: session.role,

  get isLoggedIn() {
    return Boolean(this.token)
  },

  get isAdmin() {
    return this.role === 1
  },

  login(token) {
    const parsed = parseToken(token)

    this.token = token
    this.userId = parsed.userId || 0
    this.username = parsed.username || ''
    this.role = parsed.role || 0

    persistSession({
      userId: this.userId,
      username: this.username,
      role: this.role,
    })
  },

  setToken(token) {
    const parsed = parseToken(token)

    this.token = token
    this.userId = parsed.userId || this.userId || 0
    this.username = parsed.username || this.username || ''
    this.role = parsed.role || this.role || 0

    persistSession({
      userId: this.userId,
      username: this.username,
      role: this.role,
    })
  },

  hydrateProfile(profile) {
    this.userId = profile.id || this.userId
    this.username = profile.username || this.username
    this.role = profile.role ?? this.role

    persistSession({
      userId: this.userId,
      username: this.username,
      role: this.role,
    })
  },

  logout() {
    this.token = ''
    this.userId = 0
    this.username = ''
    this.role = 0
    clearSession()
  },
})
