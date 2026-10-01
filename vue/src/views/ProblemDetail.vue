<template>
  <div class="page">
    <router-link to="/" class="back-link">← 返回题库</router-link>
    <section v-if="loading" class="loading-state">
      <strong>题目详情加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else-if="problem">
      <section class="page-hero problem-hero">
        <div class="problem-hero-main">
          <span class="eyebrow">Problem #{{ problem.id }}</span>
          <div class="page-title">
            <div>
              <h1>{{ problem.title }}</h1>
              <p class="page-subtitle">读懂题意，写下解法，用一次提交验证你的思考。</p>
            </div>
          </div>
          <div class="cluster">
            <span class="pill">时间限制 {{ problem.timeLimit }} ms</span>
            <span class="pill">内存限制 {{ problem.memoryLimit }} MB</span>
            <span class="pill">提交 {{ problem.submitCount }}</span>
            <span class="pill">通过率 {{ getAcceptanceRate(problem) }}%</span>
          </div>
          <div class="cluster">
            <span v-for="tag in problem.tags" :key="tag.id" class="mini-tag">{{ tag.name }}</span>
            <span v-if="!problem.tags.length" class="mini-tag">未分类</span>
          </div>
        </div>

        <aside class="hero-side-panel">
          <div class="hero-side-block">
            <span class="meta-label">YOUR PROGRESS</span>
            <strong>{{ problem.isAc ? '已通过这道题' : '准备好挑战了吗' }}</strong>
            <p>{{ problem.isAc ? '该账号已经通过这道题，可以继续优化代码或做复盘。' : '建议先读题并确认边界条件，再在右侧编辑器里提交第一版解法。' }}</p>
          </div>
        </aside>
      </section>

      <section class="detail-grid">
        <article class="card stack">
          <div class="section-title">
            <h2>题目描述</h2>
            <span class="muted">PROBLEM STATEMENT</span>
          </div>
          <div class="description-body" v-html="renderedDescription"></div>
        </article>

        <article class="card stack submit-panel">
          <div class="section-title">
            <h2>在线提交</h2>
            <span class="muted">CODE & SUBMIT</span>
          </div>

          <div v-if="!store.isLoggedIn" class="empty-state compact-state">
            <strong>登录后才能提交代码</strong>
            <span class="muted">登录后即可编写代码、提交判题并记录你的解题进度。</span>
            <router-link :to="{ path: '/login', query: { redirect: route.fullPath } }" class="btn btn-primary">去登录</router-link>
          </div>

          <template v-else>
            <div class="field">
              <label for="language">编程语言</label>
              <select id="language" v-model="language" class="select">
                <option value="go">Go</option>
              </select>
            </div>

            <div class="field">
              <label for="code">代码编辑器</label>
              <textarea
                id="code"
                v-model="code"
                class="textarea mono code-editor"
                spellcheck="false" autocapitalize="off" autocomplete="off"
                :placeholder="placeholderByLanguage[language]"
              ></textarea>
            </div>

            <div class="cluster">
              <button class="btn btn-primary" :disabled="submitting || !code.trim()" @click="handleSubmit">
                <span v-if="submitting" class="spinner"></span><span v-if="submitting">提交中…</span>
                <span v-else>提交判题</span>
              </button>
              <button class="btn btn-outline" @click="code = placeholderByLanguage[language]">插入模板</button>
            </div>

            <div v-if="submitState" role="status" aria-live="polite" class="submit-feedback" :class="submitState.kind">
              <div>
                <strong>{{ submitState.title }}</strong>
                <p>{{ submitState.message }}</p>
              </div>
              <router-link
                v-if="submitState.submissionId"
                :to="`/submissions/${submitState.submissionId}`"
                class="btn btn-ghost btn-sm"
              >
                查看提交详情
              </router-link>
            </div>
          </template>
        </article>
      </section>
    </template>

    <section v-else class="empty-state">
      <strong>题目不存在</strong>
      <span class="muted">可能已经被删除，或者当前 ID 无法获取到有效题面。</span>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { createEventsTicket, getErrorMessage, getProblemDetail, getSubmission, submitCode } from '../api'
import { store } from '../store'
import { getAcceptanceRate } from '../utils/normalizers'

const route = useRoute()
const loading = ref(true)
const submitting = ref(false)
const problem = ref(null)
const language = ref('go')
const code = ref('')
const submitState = ref(null)
const currentSubmissionId = ref(0)

let socket = null
let pollTimer = null
let socketFallbackTimer = null
let suppressSocketFallback = false

const SOCKET_FALLBACK_DELAY_MS = 4000
const POLL_INTERVAL_MS = 2000

const placeholderByLanguage = {
  go: 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello Gojo")\n}',
}

const renderedDescription = computed(() => {
  const text = problem.value?.description || ''
  const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return escaped.replace(/```([\s\S]*?)```/g, '<pre class="code-block">$1</pre>').replace(/\n/g, '<br>')
})

function isSubmissionPending() {
  return currentSubmissionId.value > 0 && submitState.value?.kind === 'pending'
}

async function fetchProblem() {
  loading.value = true

  try {
    problem.value = await getProblemDetail(route.params.id)
  } catch {
    problem.value = null
  } finally {
    loading.value = false
  }
}

function closeSocket() {
  if (socket) {
    suppressSocketFallback = true
    socket.close()
    socket = null
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function stopSocketFallbackTimer() {
  if (socketFallbackTimer) {
    clearTimeout(socketFallbackTimer)
    socketFallbackTimer = null
  }
}

function startPolling() {
  if (!isSubmissionPending() || pollTimer) {
    return
  }

  pollTimer = setInterval(refreshSubmission, POLL_INTERVAL_MS)
}

async function refreshSubmission() {
  if (!currentSubmissionId.value) {
    return
  }

  try {
    const submission = await getSubmission(currentSubmissionId.value)
    if (submission.status !== 'Pending') {
      submitState.value = {
        kind: submission.status === 'AC' ? 'success' : 'warning',
        title: `判题完成：${submission.status}`,
        message: submission.actualOutput || '可以点击下方按钮查看完整判题信息。',
        submissionId: submission.id,
      }
      stopSocketFallbackTimer()
      stopPolling()
      closeSocket()
      fetchProblem()
    }
  } catch {
    startPolling()
  }
}

async function openSocket(submissionId) {
  if (!store.isLoggedIn) {
    startPolling()
    return
  }

  closeSocket()
  stopSocketFallbackTimer()
  suppressSocketFallback = false

  try {
    const ticket = await createEventsTicket()
    if (!ticket || !isSubmissionPending()) {
      startPolling()
      return
    }
    socket = new EventSource(`/api/events?ticket=${encodeURIComponent(ticket)}`)
  } catch {
    startPolling()
    return
  }

  socketFallbackTimer = setTimeout(() => {
    if (socket && socket.readyState !== EventSource.OPEN && isSubmissionPending()) {
      startPolling()
    }
  }, SOCKET_FALLBACK_DELAY_MS)

  socket.onopen = () => {
    stopSocketFallbackTimer()
    stopPolling()
  }

  socket.addEventListener('judge_result', (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (Number(payload.submission_id) === Number(submissionId)) {
        void refreshSubmission()
      }
    } catch {}
  })

  socket.onerror = () => {
    if (socket) {
      suppressSocketFallback = true
      socket.close()
      socket = null
    }
    if (isSubmissionPending()) {
      startPolling()
    }
  }

  socket.onclose = () => {
    stopSocketFallbackTimer()
    const shouldFallback = !suppressSocketFallback && isSubmissionPending()
    suppressSocketFallback = false
    socket = null
    if (shouldFallback) {
      startPolling()
    }
  }
}
async function handleSubmit() {
  submitting.value = true
  submitState.value = null

  try {
    const result = await submitCode({
      problem_id: problem.value.id,
      language: language.value,
      code: code.value,
    })

    currentSubmissionId.value = result.submissionId
    submitState.value = {
      kind: 'pending',
      title: '提交成功，正在排队判题',
      message: result.message,
      submissionId: result.submissionId,
    }

    stopPolling()
    openSocket(result.submissionId)
  } catch (requestError) {
    submitState.value = {
      kind: 'danger',
      title: '提交失败',
      message: getErrorMessage(requestError, '后端没有接受这次提交。'),
      submissionId: 0,
    }
  } finally {
    submitting.value = false
  }
}

onMounted(fetchProblem)
onUnmounted(() => {
  stopSocketFallbackTimer()
  closeSocket()
  stopPolling()
})
</script>

<style scoped>
.back-link {
  font-size: 12px;
  color: var(--ink-faint);
  width: fit-content;
}

.back-link:hover {
  color: var(--brand);
}

.problem-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 20px;
}

.problem-hero-main {
  display: grid;
  gap: 16px;
}

.hero-side-panel {
  display: grid;
  align-content: start;
}

.hero-side-block {
  padding: 20px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: rgba(255, 255, 255, 0.72);
}

.meta-label {
  display: inline-block;
  margin-bottom: 10px;
  font-size: 11px;
  font-weight: 800;
  color: var(--ink-faint);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.hero-side-block strong {
  display: block;
  font-size: 22px;
  letter-spacing: -0.04em;
}

.hero-side-block p {
  margin: 10px 0 0;
  color: var(--ink-soft);
}

.detail-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(0, .92fr);
  gap: 18px;
}

.description-body {
  font-size: 15px;
  color: var(--ink);
}

.description-body :deep(pre) {
  margin: 18px 0;
  padding: 16px;
  overflow: auto;
  border-radius: var(--radius-sm);
  background: var(--surface-dark);
  color: #edf3ff;
}

.submit-panel {
  align-content: start;
}

.compact-state {
  padding: 28px 18px;
}

.code-editor {
  min-height: 380px;
  line-height: 1.9;
  tab-size: 4;
  background: var(--surface-dark);
  color: #edf3ff;
  border-color: rgba(255, 255, 255, 0.08);
}

.submit-feedback {
  display: grid;
  gap: 12px;
  padding: 18px;
  border-radius: var(--radius-sm);
}

.submit-feedback strong {
  display: block;
  font-size: 18px;
}

.submit-feedback p {
  margin: 6px 0 0;
  color: var(--ink-soft);
  white-space: pre-wrap;
  word-break: break-word;
}

.submit-feedback.pending {
  background: rgba(37, 99, 235, 0.1);
}

.submit-feedback.success {
  background: rgba(21, 128, 61, 0.12);
}

.submit-feedback.warning,
.submit-feedback.danger {
  background: rgba(220, 38, 38, 0.1);
}

@media (max-width: 980px) {
  .problem-hero,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>