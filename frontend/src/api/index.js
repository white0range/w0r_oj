import axios from 'axios'
import {
  normalizeLeaderboardItem,
  normalizeProblem,
  normalizeProfile,
  normalizeSubmission,
  normalizeTag,
  normalizeTestCase,
} from '../utils/normalizers'
import { store } from '../store'
import { parseToken } from '../utils/session'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
  withCredentials: true,
})

let refreshPromise = null

function getBody(response) {
  return response?.data ?? {}
}

function unwrapData(payload) {
  return payload?.data ?? payload
}

export function getErrorMessage(error, fallback = 'Request failed') {
  return error?.response?.data?.message || error?.response?.data?.error || fallback
}

async function refreshAccessToken() {
  const previous = store.token
  const body = getBody(await api.post('/refresh'))
  const accessToken = unwrapData(body).access_token
  if (!accessToken) throw new Error('Missing access token')
  if (store.token === previous) store.setToken(accessToken)
  return store.token
}

function refreshSession() {
  if (!refreshPromise) {
    refreshPromise = refreshAccessToken().finally(() => { refreshPromise = null })
  }
  return refreshPromise
}

export async function bootstrapSession() {
  if (store.token) return true
  try {
    return Boolean(await refreshSession())
  } catch {
    if (!store.token) store.logout()
    return false
  }
}

api.interceptors.request.use(async (config) => {
  let token = store.token
  if (token && ['/problems', '/leaderboard'].includes(config.url)
    && parseToken(token).expiresAt <= Date.now()) {
    try {
      token = await refreshSession()
    } catch {
      if (store.token === token) store.logout()
      token = store.token
    }
  }
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const request = error.config
    const isAuthRequest = ['/login', '/register', '/refresh', '/logout'].includes(request?.url)
    if (error.response?.status === 401 && request && !isAuthRequest && store.token) {
      if (!request._retry) {
        request._retry = true
        const previous = store.token
        try {
          const token = await refreshSession()
          if (token) {
            request.headers.Authorization = `Bearer ${token}`
            return api(request)
          }
        } catch {
          if (store.token === previous) store.logout()
        }
      } else if (request.headers.Authorization === `Bearer ${store.token}`) {
        store.logout()
      }
    }
    throw error
  },
)

export async function registerUser(payload) {
  const body = getBody(await api.post('/register', payload))
  return { message: body.message || 'Register succeeded' }
}

export async function loginUser(payload) {
  const body = getBody(await api.post('/login', payload))
  const data = unwrapData(body)

  return {
    accessToken: data.access_token || '',
    message: body.message || 'Login succeeded',
  }
}

export async function logoutUser() {
  try {
    await api.post('/logout')
  } finally {
    store.logout()
  }
}

export async function getProfile() {
  const body = getBody(await api.get('/profile'))
  return normalizeProfile(unwrapData(body))
}

export async function getProblems(params = {}) {
  const body = getBody(await api.get('/problems', { params }))
  const data = unwrapData(body)

  return {
    total: Number(data.total || 0),
    page: Number(data.page || params.page || 1),
    limit: Number(data.limit || params.limit || 12),
    tagId: data.tag_id || params.tag_id || '',
    message: body.message || '',
    items: (data.items || []).map(normalizeProblem),
  }
}

export async function searchProblems(keyword, tags = []) {
  const body = getBody(await api.post('/problems/search', { keyword, tags }))
  const result = unwrapData(body)
  return { total: Number(result.total || 0), items: (result.data || []).map(normalizeProblem) }
}

export async function getProblemDetail(id) {
  const body = getBody(await api.get(`/problems/${id}`))
  return normalizeProblem(unwrapData(body))
}

export async function getTags() {
  const body = getBody(await api.get('/tags'))
  return (unwrapData(body) || []).map(normalizeTag)
}

export async function submitCode(payload) {
  const body = getBody(await api.post('/submit', payload))
  const data = unwrapData(body)

  return {
    submissionId: Number(data.submission_id || 0),
    status: data.status || 'Pending',
    dispatchPending: data.dispatch_pending === true,
  }
}

export async function runProblemCode(problemId, payload, signal) {
  const body = getBody(await api.post(`/problems/${problemId}/run`, payload, { signal, timeout: 35000 }))
  return unwrapData(body)
}

export async function getSubmission(id, signal) {
  const body = getBody(await api.get(`/submissions/${id}`, { signal }))
  return normalizeSubmission(unwrapData(body))
}

export async function createEventsTicket(signal) {
  const body = getBody(await api.post('/events/ticket', null, { signal }))
  return unwrapData(body).ticket || ''
}

export async function getMySubmissions(params = {}) {
  const body = getBody(await api.get('/my-submissions', { params }))
  const data = unwrapData(body)

  return {
    total: Number(data.total || 0),
    page: Number(data.page || params.page || 1),
    limit: Number(data.limit || params.limit || 20),
    items: (data.items || []).map(normalizeSubmission),
  }
}

export async function getLeaderboard() {
  const body = getBody(await api.get('/leaderboard'))
  const data = unwrapData(body)

  return {
    top50: (data.top_50 || []).map(normalizeLeaderboardItem),
    myRank: Number(data.my_rank ?? -1),
    myScore: Number(data.my_score ?? 0),
  }
}

export async function listChatSessions(params = {}) {
  const body = getBody(await api.get('/chat/sessions', { params }))
  const data = unwrapData(body)
  return data.items || []
}

export async function createChatSession(payload = {}) {
  const body = getBody(await api.post('/chat/sessions', payload))
  return unwrapData(body)
}

export async function deleteChatSession(id) {
  const body = getBody(await api.delete(`/chat/sessions/${id}`))
  return unwrapData(body)
}

export async function getChatSession(id) {
  const body = getBody(await api.get(`/chat/sessions/${id}`))
  return unwrapData(body)
}

export async function getChatMessages(sessionId, params = {}) {
  const body = getBody(await api.get(`/chat/sessions/${sessionId}/messages`, { params }))
  const data = unwrapData(body)
  return data.items || []
}

export async function sendChatMessage(sessionId, payload) {
  const body = getBody(await api.post(`/chat/sessions/${sessionId}/messages`, payload))
  return unwrapData(body)
}

export async function getChatTurn(id) {
  const body = getBody(await api.get(`/chat/turns/${id}`))
  return unwrapData(body)
}

export async function createChatStreamTicket(id) {
  const body = getBody(await api.post(`/chat/turns/${id}/stream-ticket`))
  return unwrapData(body).ticket || ''
}

export async function submitChatPlanFeedback(turnId, payload) {
  const body = getBody(await api.post(`/chat/turns/${turnId}/feedback`, payload))
  return unwrapData(body)
}

export async function getChatPlanFeedback(turnId) {
  const body = getBody(await api.get(`/chat/turns/${turnId}/feedback`))
  return unwrapData(body)
}

export async function adminGetUsers(params = {}) {
  const body = getBody(await api.get('/admin/users', { params }))
  const data = unwrapData(body)
  return data.items || []
}

export async function adminBanUser(id, payload) {
  const body = getBody(await api.post(`/admin/users/${id}/ban`, payload))
  return unwrapData(body)
}

export async function adminUnbanUser(id) {
  const body = getBody(await api.post(`/admin/users/${id}/unban`))
  return unwrapData(body)
}

export async function adminCreateProblem(payload) {
  const body = getBody(await api.post('/admin/problems', payload))
  const data = unwrapData(body)

  return {
    problemId: Number(data.problem_id || 0),
    message: body.message || 'Create problem succeeded',
  }
}

export async function adminUpdateProblem(id, payload) {
  const body = getBody(await api.put(`/admin/problems/${id}`, payload))
  return { message: body.message || 'Update problem succeeded' }
}

export async function adminDeleteProblem(id) {
  const body = getBody(await api.delete(`/admin/problems/${id}`))
  return { message: body.message || 'Delete problem succeeded' }
}

export async function adminGetTestCases(id, params = {}) {
  const body = getBody(await api.get(`/admin/problems/${id}/cases`, { params }))
  const data = unwrapData(body)

  return {
    total: Number(data.total || 0),
    page: Number(data.page || params.page || 1),
    limit: Number(data.limit || params.limit || 20),
    items: (data.items || []).map(normalizeTestCase),
  }
}

export async function adminAddTestCase(id, payload) {
  const body = getBody(await api.post(`/admin/problems/${id}/cases`, payload))
  const data = unwrapData(body)

  return {
    caseId: Number(data.case_id || 0),
    message: body.message || 'Add test case succeeded',
  }
}

export async function adminDeleteTestCase(caseId) {
  const body = getBody(await api.delete(`/admin/problems/cases/${caseId}`))
  return { message: body.message || 'Delete test case succeeded' }
}

export async function adminCreateTag(payload) {
  const body = getBody(await api.post('/admin/tags', payload))

  return {
    tag: normalizeTag(unwrapData(body)),
    message: body.message || 'Create tag succeeded',
  }
}

export async function adminDeleteTag(id) {
  const body = getBody(await api.delete(`/admin/tags/${id}`))
  return { message: body.message || 'Delete tag succeeded' }
}

export async function adminUpdateProblemTags(id, payload) {
  const body = getBody(await api.put(`/admin/problems/${id}/tags`, payload))
  return { message: body.message || 'Update tags succeeded' }
}

export default api

