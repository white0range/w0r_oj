<template>
  <div class="study-shell">
    <header class="study-header">
      <div class="study-header-copy">
        <span class="section-kicker">AI Study Assistant</span>
        <h1>AI 学习助手</h1>
        <p>围绕刷题计划、算法问题和题目检索做连续对话，历史上下文会自动保留在当前会话里。</p>
      </div>
      <div class="study-header-meta">
        <span class="pill">{{ sessionStatusLabel }}</span>
        <span class="pill" :class="streamBadgeClass">{{ streamLabel }}</span>
        <span v-if="currentTurn?.id" class="pill">Turn #{{ currentTurn.id }}</span>
      </div>
    </header>

    <section class="study-console">
      <aside class="study-sidebar">
        <div class="sidebar-head">
          <div>
            <span class="section-kicker">Sessions</span>
            <h2>会话</h2>
          </div>
          <div class="cluster sidebar-actions">
            <button class="btn btn-primary btn-sm" :disabled="creatingSession" @click="handleCreateSession">
              <span v-if="creatingSession" class="spinner"></span>
              <span v-else>新建</span>
            </button>
            <button class="btn btn-outline btn-sm" :disabled="loadingSessions" @click="loadSessions(true)">
              <span v-if="loadingSessions" class="spinner spinner-dark"></span>
              <span v-else>刷新</span>
            </button>
          </div>
        </div>

        <div v-if="sessionMessage" class="auth-message" :class="sessionMessageType === 'error' ? 'auth-error' : 'auth-success'">
          {{ sessionMessage }}
        </div>

        <div class="session-list">
          <template v-if="sessions.length">
            <button
              v-for="session in sessions"
              :key="session.id"
              class="session-item"
              :class="{ active: session.id === activeSessionId }"
              @click="selectSession(session.id)"
            >
              <div class="session-item-head">
                <strong>{{ sessionTitle(session) }}</strong>
                <span class="session-id">#{{ session.id }}</span>
              </div>
              <span class="session-time">{{ formatDate(session.last_message_at || session.updated_at || session.created_at) }}</span>
              <span v-if="session.summary_text" class="session-memory">已生成摘要记忆</span>
            </button>
          </template>

          <div v-else class="empty-state compact-state">
            <strong>还没有会话</strong>
            <span class="muted">创建一个会话，开始记录你的训练过程和问题。</span>
          </div>
        </div>
      </aside>

      <section class="study-stage">
        <div class="stage-topbar">
          <div>
            <h2>{{ activeSession ? sessionTitle(activeSession) : '新的对话' }}</h2>
            <p>{{ sessionStatusDescription }}</p>
          </div>
          <div class="stage-pills">
            <span class="pill">{{ activeSession ? `Session #${activeSession.id}` : '未选择会话' }}</span>
            <span v-if="activeSession?.status" class="pill">{{ activeSession.status }}</span>
          </div>
        </div>

        <div v-if="activeSession?.summary_text" class="memory-banner">
          <div class="memory-head">
            <span class="section-kicker">Session Memory</span>
            <span class="memory-label">系统摘要</span>
          </div>
          <p>{{ activeSession.summary_text }}</p>
        </div>

        <div ref="messageListRef" class="message-scroll">
          <template v-if="normalizedMessages.length">
            <article
              v-for="message in normalizedMessages"
              :key="message.id"
              class="message-row"
              :class="message.role === 'user' ? 'message-row-user' : 'message-row-assistant'"
            >
              <div class="message-avatar" :class="message.role === 'user' ? 'message-avatar-user' : 'message-avatar-assistant'">
                {{ message.role === 'user' ? '你' : 'AI' }}
              </div>

              <div class="message-bubble" :class="message.role === 'user' ? 'message-bubble-user' : 'message-bubble-assistant'">
                <div class="message-meta">
                  <strong>{{ message.role === 'user' ? '你' : '学习助手' }}</strong>
                  <span>{{ formatDate(message.created_at) }}</span>
                </div>

                <div class="message-body">
                  <template v-if="message.role === 'assistant' && message.parsed">
                    <div class="message-text assistant-summary">
                      {{ message.parsed.study_plan_summary || message.content }}
                    </div>

                    <div v-if="message.parsed.weak_tags.length" class="assistant-section">
                      <span class="assistant-label">薄弱标签</span>
                      <div class="cluster">
                        <span v-for="tag in message.parsed.weak_tags" :key="`${message.id}-${tag}`" class="pill weak-pill">{{ tag }}</span>
                      </div>
                    </div>

                    <div v-if="message.parsed.recommended_problems.length" class="assistant-section">
                      <span class="assistant-label">推荐题目</span>
                      <div class="recommend-grid">
                        <router-link
                          v-for="problem in message.parsed.recommended_problems"
                          :key="`${message.id}-${problem.problem_id}`"
                          :to="`/problems/${problem.problem_id}`"
                          class="recommend-card"
                        >
                          <span class="mini-tag">#{{ problem.problem_id }}</span>
                          <strong>{{ problem.title }}</strong>
                          <p>{{ problem.reason }}</p>
                          <span class="recommend-link">进入题目</span>
                        </router-link>
                      </div>
                    </div>
                  </template>

                  <template v-else>
                    <div class="message-text">{{ message.content }}</div>
                  </template>
                </div>
              </div>
            </article>
          </template>

          <div v-else-if="loadingMessages" class="loading-state inline-state">
            <span class="spinner spinner-dark"></span>
            <span class="muted">正在加载会话消息...</span>
          </div>

          <div v-else class="welcome-panel">
            <div class="welcome-copy">
              <span class="section-kicker">开始对话</span>
              <h3>开始一段新的学习对话</h3>
              <p>可以直接问算法问题、回忆模糊题目，或者让助手帮你规划下一轮训练。</p>
            </div>
            <div class="prompt-grid">
              <button
                v-for="preset in promptPresets"
                :key="preset"
                class="prompt-card"
                @click="draft = preset"
              >
                {{ preset }}
              </button>
            </div>
          </div>

          <div v-if="turnPending" class="assistant-pending">
            <div class="message-avatar message-avatar-assistant">AI</div>
            <div class="pending-bubble">
              <strong>正在生成回复</strong>
              <span>{{ streamMessage || '完成后会自动写入当前会话。' }}</span>
            </div>
          </div>
        </div>

        <div class="composer-shell">
          <div v-if="messageError" class="auth-message auth-error">{{ messageError }}</div>

          <textarea
            id="study-chat-input"
            v-model.trim="draft"
            class="textarea composer-input"
            placeholder="输入你的问题，例如：我最近图论建模总是卡住，先帮我分析问题，再给我安排一轮练习。"
            @keydown.enter.exact.prevent="handleSendMessage"
          ></textarea>

          <div class="composer-footer">
            <div class="composer-hints">
              <span>{{ sessionStatusDescription }}</span>
            </div>
            <div class="cluster composer-actions">
              <button class="btn btn-outline" :disabled="loadingMessages || !activeSessionId" @click="reloadActiveSession">
                <span v-if="loadingMessages" class="spinner spinner-dark"></span>
                <span v-else>刷新消息</span>
              </button>
              <button class="btn btn-primary" :disabled="sending || !draft.trim()" @click="handleSendMessage">
                <span v-if="sending" class="spinner"></span>
                <span v-else>发送</span>
              </button>
            </div>
          </div>
        </div>
      </section>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createStudyPlanSession,
  getErrorMessage,
  getStudyPlanMessages,
  getStudyPlanTurn,
  listStudyPlanSessions,
  sendStudyPlanMessage,
} from '../api'
import { store } from '../store'

const STUDY_PLAN_SESSION_KEY = 'gojo:studyPlanSessionId'
const ACTIVE_TURN_STATUSES = ['pending', 'running']
const TERMINAL_TURN_STATUSES = ['succeeded', 'failed']

const route = useRoute()
const router = useRouter()

const promptPresets = [
  '我最近动态规划总在状态设计上卡住，先帮我定位问题。',
  '准备笔试，给我安排一轮偏图论和最短路的训练。',
  '我记得有道题和饭量有关，帮我回忆一下是哪道题。',
]

const sessions = ref([])
const activeSessionId = ref(0)
const messages = ref([])
const currentTurn = ref(null)
const draft = ref('')
const loadingSessions = ref(false)
const loadingMessages = ref(false)
const creatingSession = ref(false)
const sending = ref(false)
const sessionMessage = ref('')
const sessionMessageType = ref('success')
const messageError = ref('')
const streamState = ref('idle')
const streamMessage = ref('')
const messageListRef = ref(null)

let turnStream = null
let turnStreamTurnId = 0

const activeSession = computed(() => sessions.value.find((item) => item.id === activeSessionId.value) || null)

const normalizedMessages = computed(() => messages.value.map((message) => ({
  ...message,
  parsed: parseAssistantPayload(message),
})))

const turnPending = computed(() => ACTIVE_TURN_STATUSES.includes(currentTurn.value?.status || ''))

const sessionStatusLabel = computed(() => {
  if (turnPending.value) {
    return '回复生成中'
  }
  if (activeSession.value) {
    return '会话已就绪'
  }
  if (sessions.value.length) {
    return '等待选择会话'
  }
  return '尚未开始'
})

const sessionStatusDescription = computed(() => {
  if (turnPending.value) {
    return '当前回复正在生成，完成后会自动回写到会话记录。'
  }
  if (activeSession.value) {
    return '你可以在这个会话里连续追问，系统会自动保留上下文。'
  }
  return '创建一个会话，开始记录你的学习问题与训练安排。'
})

const streamLabel = computed(() => {
  switch (streamState.value) {
    case 'connecting':
      return '建立连接中'
    case 'connected':
      return '实时同步中'
    case 'reconnecting':
      return '正在重连'
    case 'error':
      return '连接异常'
    default:
      return '空闲'
  }
})

const streamBadgeClass = computed(() => {
  switch (streamState.value) {
    case 'connected':
      return 'stream-pill stream-pill-live'
    case 'connecting':
    case 'reconnecting':
      return 'stream-pill stream-pill-warn'
    case 'error':
      return 'stream-pill stream-pill-error'
    default:
      return 'stream-pill'
  }
})

function sessionTitle(session) {
  return session?.title?.trim() || '新会话'
}

function formatDate(value) {
  if (!value) {
    return '未记录'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }

  return date.toLocaleString('zh-CN', { hour12: false })
}

function persistSessionId(sessionId) {
  activeSessionId.value = sessionId
  if (sessionId > 0) {
    localStorage.setItem(STUDY_PLAN_SESSION_KEY, String(sessionId))
  } else {
    localStorage.removeItem(STUDY_PLAN_SESSION_KEY)
  }

  const nextQuery = { ...route.query }
  if (sessionId > 0) {
    nextQuery.session = String(sessionId)
  } else {
    delete nextQuery.session
  }
  router.replace({ query: nextQuery })
}

function getStoredSessionId() {
  const querySessionId = Number(route.query.session || 0)
  if (querySessionId > 0) {
    return querySessionId
  }

  const storedSessionId = Number(localStorage.getItem(STUDY_PLAN_SESSION_KEY) || 0)
  return storedSessionId > 0 ? storedSessionId : 0
}

function closeTurnStream(resetState = true) {
  if (turnStream) {
    turnStream.close()
    turnStream = null
  }
  turnStreamTurnId = 0

  if (resetState) {
    streamState.value = 'idle'
    streamMessage.value = ''
  }
}

function scrollMessagesToBottom() {
  nextTick(() => {
    const element = messageListRef.value
    if (!element) {
      return
    }
    element.scrollTop = element.scrollHeight
  })
}

function parseAssistantPayload(message) {
  if (message?.role !== 'assistant') {
    return null
  }

  const raw = message.structured_payload || ''
  if (!raw) {
    return null
  }

  try {
    const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
    return {
      weak_tags: Array.isArray(parsed?.weak_tags) ? parsed.weak_tags : [],
      recommended_problems: Array.isArray(parsed?.recommended_problems) ? parsed.recommended_problems : [],
      study_plan_summary: parsed?.study_plan_summary || '',
    }
  } catch {
    return null
  }
}

async function ensureActiveSession() {
  if (activeSessionId.value > 0) {
    return activeSessionId.value
  }

  const created = await createStudyPlanSession({ title: '' })
  await loadSessions(false, created.id)
  persistSessionId(created.id)
  return created.id
}

async function loadSessions(showMessage = false, preferredSessionId = 0) {
  loadingSessions.value = true

  try {
    const items = await listStudyPlanSessions({ limit: 50 })
    sessions.value = items

    const requestedSessionId = preferredSessionId || activeSessionId.value || getStoredSessionId()
    if (requestedSessionId > 0 && items.some((item) => item.id === requestedSessionId)) {
      activeSessionId.value = requestedSessionId
    } else if (items.length > 0) {
      activeSessionId.value = items[0].id
    } else {
      activeSessionId.value = 0
    }

    if (activeSessionId.value > 0) {
      persistSessionId(activeSessionId.value)
    }

    if (showMessage) {
      sessionMessage.value = '会话列表已刷新。'
      sessionMessageType.value = 'success'
    }
  } catch (error) {
    sessionMessage.value = getErrorMessage(error, '读取会话列表失败。')
    sessionMessageType.value = 'error'
  } finally {
    loadingSessions.value = false
  }
}

async function syncPendingTurn() {
  closeTurnStream(false)
  currentTurn.value = null

  const lastTurnId = [...messages.value].reverse().find((item) => Number(item.turn_id || 0) > 0)?.turn_id || 0
  if (!lastTurnId) {
    streamState.value = 'idle'
    streamMessage.value = ''
    return
  }

  try {
    const turn = await getStudyPlanTurn(lastTurnId)
    currentTurn.value = turn
    if (ACTIVE_TURN_STATUSES.includes(turn.status || '')) {
      connectTurnStream(turn.id)
      return
    }
  } catch {
    currentTurn.value = null
  }

  streamState.value = 'idle'
  streamMessage.value = ''
}

async function loadMessages(sessionId = activeSessionId.value) {
  if (!sessionId) {
    messages.value = []
    currentTurn.value = null
    closeTurnStream()
    return
  }

  loadingMessages.value = true
  messageError.value = ''

  try {
    const items = await getStudyPlanMessages(sessionId)
    messages.value = items
    await syncPendingTurn()
    scrollMessagesToBottom()
  } catch (error) {
    messageError.value = getErrorMessage(error, '读取会话消息失败。')
  } finally {
    loadingMessages.value = false
  }
}

async function selectSession(sessionId) {
  if (!sessionId || sessionId === activeSessionId.value) {
    if (sessionId) {
      await loadMessages(sessionId)
    }
    return
  }

  persistSessionId(sessionId)
  await loadMessages(sessionId)
}

async function reloadActiveSession() {
  if (!activeSessionId.value) {
    return
  }
  await loadSessions(false, activeSessionId.value)
  await loadMessages(activeSessionId.value)
}

function connectTurnStream(turnId) {
  if (!turnId || (turnStream && turnStreamTurnId === turnId)) {
    return
  }

  const token = store.token
  if (!token) {
    streamState.value = 'error'
    streamMessage.value = '缺少登录凭证，无法建立实时连接。'
    return
  }

  closeTurnStream(false)

  const source = new EventSource(`/api/study-plan/turns/${turnId}/stream?token=${encodeURIComponent(token)}`)
  turnStream = source
  turnStreamTurnId = turnId
  streamState.value = 'connecting'
  streamMessage.value = '正在建立回复同步连接...'

  source.onopen = () => {
    if (turnStream !== source) {
      return
    }
    streamState.value = 'connected'
    streamMessage.value = '连接已建立，当前回复会自动同步。'
  }

  source.onmessage = async (event) => {
    if (turnStream !== source) {
      return
    }

    try {
      const nextTurn = JSON.parse(event.data)
      currentTurn.value = nextTurn

      if (TERMINAL_TURN_STATUSES.includes(nextTurn.status || '')) {
        closeTurnStream(false)
        streamState.value = 'idle'
        streamMessage.value = nextTurn.status === 'succeeded' ? '回复已生成完成。' : '这次回复生成失败，请重试。'
        await loadSessions(false, activeSessionId.value)
        await loadMessages(activeSessionId.value)
      }
    } catch {
      closeTurnStream(false)
      streamState.value = 'error'
      streamMessage.value = '实时数据解析失败，请刷新当前会话。'
    }
  }

  source.onerror = () => {
    if (turnStream !== source) {
      return
    }

    if (TERMINAL_TURN_STATUSES.includes(currentTurn.value?.status || '')) {
      closeTurnStream(false)
      streamState.value = 'idle'
      return
    }

    if (source.readyState === EventSource.CLOSED) {
      closeTurnStream(false)
      streamState.value = 'error'
      streamMessage.value = '实时连接已关闭，请刷新当前会话。'
      return
    }

    streamState.value = 'reconnecting'
    streamMessage.value = '连接短暂中断，正在自动重连...'
  }
}

async function handleCreateSession() {
  creatingSession.value = true
  sessionMessage.value = ''

  try {
    const created = await createStudyPlanSession({ title: '' })
    await loadSessions(false, created.id)
    persistSessionId(created.id)
    messages.value = []
    currentTurn.value = null
    closeTurnStream()
    draft.value = ''
    sessionMessage.value = '新会话已创建。'
    sessionMessageType.value = 'success'
  } catch (error) {
    sessionMessage.value = getErrorMessage(error, '创建会话失败。')
    sessionMessageType.value = 'error'
  } finally {
    creatingSession.value = false
  }
}

async function handleSendMessage() {
  const content = draft.value.trim()
  if (!content) {
    messageError.value = '先输入一条消息再发送。'
    return
  }

  sending.value = true
  messageError.value = ''
  sessionMessage.value = ''

  try {
    const sessionId = await ensureActiveSession()
    const createdTurn = await sendStudyPlanMessage(sessionId, { content })
    draft.value = ''
    await loadSessions(false, sessionId)
    await loadMessages(sessionId)
    currentTurn.value = {
      id: createdTurn.turn_id,
      status: createdTurn.status,
      model: createdTurn.model,
      session_id: createdTurn.session_id,
    }
    connectTurnStream(createdTurn.turn_id)
  } catch (error) {
    messageError.value = getErrorMessage(error, '发送消息失败。')
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  await loadSessions(false)
  if (activeSessionId.value) {
    await loadMessages(activeSessionId.value)
  }
})

onUnmounted(() => {
  closeTurnStream()
})
</script>

<style scoped>
.study-shell {
  display: grid;
  gap: 18px;
}

.study-header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
  padding: 6px 4px 2px;
}

.study-header-copy {
  display: grid;
  gap: 10px;
}

.study-header-copy h1 {
  margin: 0;
  font-size: clamp(32px, 4vw, 46px);
  letter-spacing: -0.05em;
}

.study-header-copy p {
  margin: 0;
  color: var(--ink-soft);
  max-width: 760px;
}

.study-header-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.study-console {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 18px;
  min-height: 76vh;
}

.study-sidebar,
.study-stage {
  border: 1px solid rgba(15, 23, 40, 0.08);
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.84);
  box-shadow: var(--shadow-md);
}

.study-sidebar {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 14px;
  padding: 18px;
}

.sidebar-head {
  display: grid;
  gap: 12px;
}

.sidebar-head h2,
.stage-topbar h2,
.welcome-copy h3 {
  margin: 6px 0 0;
  font-size: 24px;
  letter-spacing: -0.04em;
}

.sidebar-actions {
  gap: 8px;
}

.session-list {
  display: grid;
  gap: 10px;
  min-height: 0;
  max-height: calc(76vh - 120px);
  overflow: auto;
  padding-right: 4px;
}

.session-item {
  display: grid;
  gap: 8px;
  text-align: left;
  padding: 14px 16px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  border-radius: 18px;
  background: rgba(245, 248, 255, 0.82);
  transition: transform var(--transition), border-color var(--transition), background var(--transition), box-shadow var(--transition);
}

.session-item:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.22);
  box-shadow: var(--shadow-sm);
}

.session-item.active {
  border-color: rgba(37, 99, 235, 0.28);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(59, 130, 246, 0.05));
}

.session-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.session-id,
.session-time,
.session-memory,
.stage-topbar p,
.memory-label,
.message-meta span,
.composer-hints {
  color: var(--ink-soft);
}

.session-id,
.session-memory {
  font-size: 12px;
}

.session-time {
  font-size: 13px;
}

.session-memory {
  font-weight: 700;
}

.study-stage {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  gap: 16px;
  padding: 18px;
}

.stage-topbar {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 4px 2px 0;
}

.stage-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.memory-banner {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
  border-radius: 20px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  background: linear-gradient(135deg, rgba(15, 23, 40, 0.035), rgba(37, 99, 235, 0.045));
}

.memory-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.memory-banner p {
  margin: 0;
  white-space: pre-wrap;
  color: var(--ink-soft);
}

.message-scroll {
  display: grid;
  align-content: start;
  gap: 18px;
  min-height: 0;
  max-height: calc(76vh - 260px);
  overflow: auto;
  padding: 4px 6px 4px 2px;
}

.message-row,
.assistant-pending {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}

.message-row-user {
  grid-template-columns: minmax(0, 1fr) 44px;
}

.message-row-user .message-avatar {
  order: 2;
}

.message-row-user .message-bubble {
  order: 1;
  justify-self: end;
}

.message-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 800;
}

.message-avatar-user {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.98), rgba(29, 78, 216, 0.82));
  color: #f8fbff;
}

.message-avatar-assistant {
  background: rgba(15, 23, 40, 0.08);
  color: var(--ink);
}

.message-bubble,
.pending-bubble {
  display: grid;
  gap: 12px;
  width: min(100%, 860px);
  padding: 16px 18px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  border-radius: 24px;
  box-shadow: var(--shadow-sm);
}

.message-bubble-user {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.98), rgba(59, 130, 246, 0.88));
  color: #f8fbff;
}

.message-bubble-user .message-meta span,
.message-bubble-user .message-text {
  color: rgba(248, 251, 255, 0.82);
}

.message-bubble-assistant,
.pending-bubble {
  background: rgba(255, 255, 255, 0.92);
}

.message-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.message-body,
.assistant-section {
  display: grid;
  gap: 12px;
}

.message-text {
  white-space: pre-wrap;
  line-height: 1.75;
}

.assistant-summary {
  font-size: 15px;
}

.assistant-label {
  font-size: 13px;
  font-weight: 800;
  color: var(--ink-soft);
}

.recommend-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.recommend-card {
  display: grid;
  gap: 10px;
  padding: 16px;
  border-radius: 20px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  background: rgba(245, 248, 255, 0.92);
  transition: transform var(--transition), box-shadow var(--transition), border-color var(--transition);
}

.recommend-card:hover {
  transform: translateY(-2px);
  border-color: rgba(37, 99, 235, 0.24);
  box-shadow: var(--shadow-sm);
}

.recommend-card p {
  margin: 0;
  color: var(--ink-soft);
}

.recommend-link {
  color: var(--brand-deep);
  font-weight: 800;
}

.welcome-panel {
  display: grid;
  gap: 20px;
  padding: 24px;
  border: 1px dashed rgba(15, 23, 40, 0.16);
  border-radius: 24px;
  background: rgba(248, 250, 255, 0.82);
}

.welcome-copy {
  display: grid;
  gap: 8px;
}

.welcome-copy p {
  margin: 0;
  color: var(--ink-soft);
}

.prompt-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.prompt-card {
  text-align: left;
  padding: 16px 18px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--ink);
  font-weight: 600;
  transition: transform var(--transition), border-color var(--transition), box-shadow var(--transition);
}

.prompt-card:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.24);
  box-shadow: var(--shadow-sm);
}

.assistant-pending {
  align-items: center;
}

.pending-bubble strong {
  font-size: 15px;
}

.pending-bubble span {
  color: var(--ink-soft);
}

.composer-shell {
  display: grid;
  gap: 12px;
  padding-top: 8px;
  border-top: 1px solid rgba(15, 23, 40, 0.08);
}

.composer-input {
  min-height: 118px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.96);
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.composer-hints {
  font-size: 13px;
}

.composer-actions {
  justify-content: flex-end;
}

.inline-state {
  justify-items: start;
  padding: 18px;
}

.weak-pill {
  background: rgba(217, 119, 6, 0.12);
  color: var(--warning);
}

.stream-pill {
  border: 1px solid rgba(15, 23, 42, 0.08);
}

.stream-pill-live {
  background: rgba(15, 118, 110, 0.12);
  color: #0f766e;
}

.stream-pill-warn {
  background: rgba(217, 119, 6, 0.12);
  color: #b45309;
}

.stream-pill-error {
  background: rgba(220, 38, 38, 0.12);
  color: #b91c1c;
}

.compact-state {
  padding: 24px 16px;
}

@media (max-width: 1100px) {
  .study-console {
    grid-template-columns: 1fr;
  }

  .session-list {
    max-height: none;
  }

  .message-scroll {
    max-height: none;
  }
}

@media (max-width: 760px) {
  .study-header,
  .stage-topbar,
  .composer-footer,
  .memory-head {
    grid-template-columns: 1fr;
    display: grid;
    align-items: start;
  }

  .message-row,
  .assistant-pending,
  .message-row-user {
    grid-template-columns: 1fr;
  }

  .message-row-user .message-avatar,
  .message-row-user .message-bubble {
    order: initial;
  }

  .message-bubble,
  .pending-bubble {
    width: 100%;
  }

  .message-avatar {
    width: 38px;
    height: 38px;
  }
}
</style>


