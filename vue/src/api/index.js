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

export function getErrorMessage(error, fallback = '请求失败') {
  return error?.response?.data?.message || error?.response?.data?.error || fallback
}

function currentHashPath() {
  const hash = window.location.hash.replace(/^#/, '')
  return hash || '/'
}

function redirectToLogin() {
  const path = currentHashPath()
  if (!path.startsWith('/login') && !path.startsWith('/register')) {
    window.location.hash = `#/login?redirect=${encodeURIComponent(path)}`
  }
}

async function refreshAccessToken() {
  const body = getBody(await api.post('/refresh'))
  const data = unwrapData(body)
  const accessToken = data.access_token || ''
  store.setToken(accessToken)
  return accessToken
}

export async function bootstrapSession() {
  if (store.token) {
    return true
  }

  try {
    await refreshAccessToken()
    return true
  } catch {
    store.logout()
    return false
  }
}

api.interceptors.request.use((config) => {
  const token = store.token

  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }

  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const status = error.response?.status
    const originalRequest = error.config || {}
    const url = String(originalRequest.url || '')
    const isAuthRequest = url.includes('/login') || url.includes('/register') || url.includes('/refresh') || url.includes('/logout')

    if (status === 401 && !isAuthRequest && !originalRequest._retry) {
      originalRequest._retry = true

      try {
        if (!refreshPromise) {
          refreshPromise = refreshAccessToken().finally(() => {
            refreshPromise = null
          })
        }

        const accessToken = await refreshPromise
        originalRequest.headers = originalRequest.headers || {}
        originalRequest.headers.Authorization = `Bearer ${accessToken}`
        return api(originalRequest)
      } catch {
        store.logout()
        redirectToLogin()
      }
    }

    if (status === 401 || status === 403) {
      if (!isAuthRequest) {
        store.logout()
        redirectToLogin()
      }
    }

    return Promise.reject(error)
  },
)

export async function registerUser(payload) {
  const body = getBody(await api.post('/register', payload))
  return { message: body.message || '注册成功' }
}

export async function loginUser(payload) {
  const body = getBody(await api.post('/login', payload))
  const data = unwrapData(body)

  return {
    accessToken: data.access_token || '',
    message: body.message || '登录成功',
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
    message: body.message || '提交成功',
  }
}

export async function getSubmission(id) {
  const body = getBody(await api.get(`/submissions/${id}`))
  return normalizeSubmission(unwrapData(body))
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

export async function createStudyPlanTask(payload) {
  const body = getBody(await api.post('/study-plan/tasks', payload))
  const data = unwrapData(body)

  return {
    taskId: Number(data.task_id || 0),
    status: data.status || 'pending',
    goal: data.goal || '',
    message: body.message || '训练计划任务已创建',
  }
}

export async function getStudyPlanTask(id) {
  const body = getBody(await api.get(`/study-plan/tasks/${id}`))
  return unwrapData(body)
}

export async function submitStudyPlanFeedback(id, payload) {
  const body = getBody(await api.post(`/study-plan/tasks/${id}/feedback`, payload))
  return unwrapData(body)
}

export async function getStudyPlanFeedback(id) {
  const body = getBody(await api.get(`/study-plan/tasks/${id}/feedback`))
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
    message: body.message || '创建成功',
  }
}

export async function adminUpdateProblem(id, payload) {
  const body = getBody(await api.put(`/admin/problems/${id}`, payload))
  return { message: body.message || '更新成功' }
}

export async function adminDeleteProblem(id) {
  const body = getBody(await api.delete(`/admin/problems/${id}`))
  return { message: body.message || '删除成功' }
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
    message: body.message || '测试用例已添加',
  }
}

export async function adminDeleteTestCase(caseId) {
  const body = getBody(await api.delete(`/admin/problems/cases/${caseId}`))
  return { message: body.message || '测试用例已删除' }
}

export async function adminCreateTag(payload) {
  const body = getBody(await api.post('/admin/tags', payload))

  return {
    tag: normalizeTag(unwrapData(body)),
    message: body.message || '标签已创建',
  }
}

export async function adminDeleteTag(id) {
  const body = getBody(await api.delete(`/admin/tags/${id}`))
  return { message: body.message || '标签已删除' }
}

export async function adminUpdateProblemTags(id, payload) {
  const body = getBody(await api.put(`/admin/problems/${id}/tags`, payload))
  return { message: body.message || '标签已更新' }
}

export default api
