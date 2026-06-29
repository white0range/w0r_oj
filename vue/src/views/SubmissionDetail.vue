<template>
  <div class="page">
    <section v-if="loading" class="loading-state">
      <strong>提交详情加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else-if="submission">
      <section class="page-hero submission-hero">
        <div>
          <span class="eyebrow">Submission #{{ submission.id }}</span>
          <div class="page-title">
            <div>
              <h1>{{ submission.status }}</h1>
              <p class="page-subtitle">查看本次提交的代码、判题输出和基础运行指标。</p>
            </div>
          </div>
          <div class="cluster">
            <span class="pill">题目 #{{ submission.problemId }}</span>
            <span class="pill">{{ submission.language.toUpperCase() }}</span>
            <span class="pill">CPU {{ submission.timeCost }} ms</span>
            <span class="pill">Memory {{ submission.memoryCost }} KB</span>
          </div>
        </div>
        <router-link to="/my-submissions" class="btn btn-outline">返回提交列表</router-link>
      </section>

      <section class="detail-grid">
        <article class="card stack">
          <div class="section-title">
            <h2>提交代码</h2>
          </div>
          <pre class="code-view mono">{{ submission.code || '后端没有返回代码内容。' }}</pre>
        </article>

        <article class="card stack">
          <div class="section-title">
            <h2>运行输出</h2>
          </div>
          <pre class="output-view mono">{{ submission.actualOutput || '当前没有输出信息。' }}</pre>
        </article>
      </section>
    </template>

    <section v-else class="empty-state">
      <strong>这条提交不存在</strong>
      <span class="muted">可能已经失效，或者你没有权限查看它。</span>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getSubmission } from '../api'
import { store } from '../store'

const route = useRoute()
const routeId = computed(() => route.params.id)
const loading = ref(true)
const submission = ref(null)

let socket = null
let pollTimer = null
let socketFallbackTimer = null
let suppressSocketFallback = false

const SOCKET_FALLBACK_DELAY_MS = 4000
const POLL_INTERVAL_MS = 2000

function isPendingSubmission() {
  return submission.value?.status === 'Pending'
}

async function fetchSubmission() {
  try {
    submission.value = await getSubmission(routeId.value)
  } catch {
    submission.value = null
  } finally {
    loading.value = false
  }

  if (!isPendingSubmission()) {
    stopSocketFallbackTimer()
    stopPolling()
    closeSocket()
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function startPolling() {
  if (!isPendingSubmission() || pollTimer) {
    return
  }

  pollTimer = setInterval(fetchSubmission, POLL_INTERVAL_MS)
}

function stopSocketFallbackTimer() {
  if (socketFallbackTimer) {
    clearTimeout(socketFallbackTimer)
    socketFallbackTimer = null
  }
}

function closeSocket() {
  if (socket) {
    suppressSocketFallback = true
    socket.close()
    socket = null
  }
}

function openSocket(submissionId) {
  const token = store.token
  if (!token) {
    startPolling()
    return
  }

  closeSocket()
  stopSocketFallbackTimer()
  suppressSocketFallback = false

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  socket = new WebSocket(`${protocol}//${window.location.host}/api/ws?token=${encodeURIComponent(token)}`)

  socketFallbackTimer = setTimeout(() => {
    if (socket && socket.readyState !== WebSocket.OPEN && isPendingSubmission()) {
      startPolling()
    }
  }, SOCKET_FALLBACK_DELAY_MS)

  socket.onopen = () => {
    stopSocketFallbackTimer()
    stopPolling()
  }

  socket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (Number(payload.submission_id) === Number(submissionId)) {
        fetchSubmission()
      }
    } catch {}
  }

  socket.onerror = () => {
    if (isPendingSubmission()) {
      startPolling()
    }
  }

  socket.onclose = () => {
    stopSocketFallbackTimer()
    const shouldFallback = !suppressSocketFallback && isPendingSubmission()
    suppressSocketFallback = false
    socket = null

    if (shouldFallback) {
      startPolling()
    }
  }
}

onMounted(async () => {
  await fetchSubmission()

  if (isPendingSubmission()) {
    openSocket(routeId.value)
  }
})

onUnmounted(() => {
  stopSocketFallbackTimer()
  closeSocket()
  stopPolling()
})
</script>

<style scoped>
.submission-hero {
  display: flex;
  justify-content: space-between;
  align-items: start;
  gap: 16px;
}

.detail-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: 1fr 1fr;
}

.code-view,
.output-view {
  margin: 0;
  padding: 18px;
  border-radius: 18px;
  background: var(--surface-dark);
  color: #eef4ff;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.output-view {
  background: #0b1324;
  color: #d5e3ff;
}

@media (max-width: 900px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }

  .submission-hero {
    flex-direction: column;
  }
}
</style>
