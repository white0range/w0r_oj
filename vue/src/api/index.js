import axios from 'axios'
import {
  normalizeLeaderboardItem,
  normalizeProblem,
  normalizeProfile,
  normalizeSubmission,
  normalizeTag,
  normalizeTestCase,
} from '../utils/normalizers'
import { clearSession } from '../utils/session'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

function getBody(response) {
  return response?.data ?? {}
}

function unwrapData(payload) {
  return payload?.data ?? payload
}

function currentHashPath() {
  const hash = window.location.hash.replace(/^#/, '')
  return hash || '/'
}

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')

  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }

  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      clearSession()

      const path = currentHashPath()
      if (!path.startsWith('/login') && !path.startsWith('/register')) {
        window.location.hash = `#/login?redirect=${encodeURIComponent(path)}`
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
  return {
    token: body.token || '',
    message: body.message || '登录成功',
  }
}

export async function getProfile() {
  const body = getBody(await api.get('/profile'))
  return normalizeProfile(unwrapData(body))
}

export async function getProblems(params = {}) {
  const body = getBody(await api.get('/problems', { params }))

  return {
    total: Number(body.total || 0),
    page: Number(body.page || params.page || 1),
    limit: Number(body.limit || params.limit || 12),
    tagId: body.tag_id || params.tag_id || '',
    message: body.message || '',
    items: (body.items || []).map(normalizeProblem),
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
  return {
    submissionId: Number(body.submission_id || 0),
    status: body.status || 'Pending',
    message: body.message || '提交成功',
  }
}

export async function getSubmission(id) {
  const body = getBody(await api.get(`/submissions/${id}`))
  return normalizeSubmission(body)
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

export async function adminCreateProblem(payload) {
  const body = getBody(await api.post('/admin/problems', payload))
  return {
    problemId: Number(body.problem_id || 0),
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
  return {
    caseId: Number(body.case_id || 0),
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
