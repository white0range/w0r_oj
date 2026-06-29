<template>
  <div class="chat-shell">
    <aside class="chat-sidebar">
      <div class="sidebar-top">
        <div class="sidebar-brand">
          <span class="sidebar-kicker">AI Study Assistant</span>
          <h1>学习计划</h1>
          <p>围绕刷题、算法问题和题目检索做连续对话。</p>
        </div>

        <div class="sidebar-toolbar">
          <button class="sidebar-primary" :disabled="creatingSession" @click="handleCreateSession">
            <span v-if="creatingSession" class="spinner"></span>
            <span v-else>新建会话</span>
          </button>
          <button class="sidebar-secondary" :disabled="loadingSessions" @click="loadSessions(true)">
            <span v-if="loadingSessions" class="spinner spinner-dark"></span>
            <span v-else>刷新</span>
          </button>
        </div>

        <div v-if="sessionMessage" class="sidebar-alert" :class="sessionMessageType === 'error' ? 'sidebar-alert-error' : 'sidebar-alert-success'">
          {{ sessionMessage }}
        </div>
      </div>

      <div class="session-scroll">
        <div v-if="sessions.length" class="session-stack">
          <button
            v-for="session in sessions"
            :key="session.id"
            class="session-row"
            :class="{ active: session.id === activeSessionId }"
            @click="selectSession(session.id)"
          >
            <div class="session-row-head">
              <strong>{{ sessionTitle(session) }}</strong>
              <span>#{{ session.id }}</span>
            </div>
            <span class="session-row-time">{{ formatDate(session.last_message_at || session.updated_at || session.created_at) }}</span>
            <span v-if="session.summary_text" class="session-row-memory">已生成摘要记忆</span>
          </button>
        </div>

        <div v-else class="sidebar-empty">
          <strong>还没有会话</strong>
          <p>创建一个新会话，开始记录你的问题和训练过程。</p>
        </div>
      </div>
    </aside>

    <section class="chat-main">
      <header class="chat-main-header">
        <div class="chat-main-copy">
          <span class="chat-main-kicker">Conversation</span>
          <h2>{{ activeSession ? sessionTitle(activeSession) : '新的对话' }}</h2>
          <p>{{ sessionStatusDescription }}</p>
        </div>

        <div class="chat-main-meta">
          <span class="meta-pill">{{ activeSession ? `Session #${activeSession.id}` : '未选择会话' }}</span>
          <span class="meta-pill" :class="streamBadgeClass">{{ streamLabel }}</span>
          <span v-if="currentTurn?.id" class="meta-pill">Turn #{{ currentTurn.id }}</span>
          <button class="danger-pill" :disabled="deletingSession || !activeSessionId || turnPending" @click="handleDeleteSession">
            <span v-if="deletingSession" class="spinner spinner-dark"></span>
            <span v-else>删除会话</span>
          </button>
        </div>
      </header>

      <div v-if="activeSession?.summary_text" class="memory-strip">
        <div class="memory-strip-head">
          <span>Session Memory</span>
          <small>系统摘要</small>
        </div>
        <p>{{ activeSession.summary_text }}</p>
      </div>

      <div ref="messageListRef" class="chat-thread">
        <template v-if="normalizedMessages.length">
          <article
            v-for="message in normalizedMessages"
            :key="message.id"
            class="message-item"
            :class="message.role === 'user' ? 'message-item-user' : 'message-item-assistant'"
          >
            <div class="message-avatar" :class="message.role === 'user' ? 'message-avatar-user' : 'message-avatar-assistant'">
              {{ message.role === 'user' ? '你' : 'AI' }}
            </div>

            <div class="message-card" :class="message.role === 'user' ? 'message-card-user' : 'message-card-assistant'">
              <div class="message-head">
                <strong>{{ message.role === 'user' ? '你' : '学习助手' }}</strong>
                <span>{{ formatDate(message.created_at) }}</span>
              </div>

              <div class="message-content">
                <template v-if="message.role === 'assistant' && message.parsed">
                  <div class="message-text assistant-summary">
                    {{ message.parsed.study_plan_summary || message.content }}
                  </div>

                  <div v-if="message.parsed.weak_tags.length" class="assistant-block">
                    <span class="assistant-label">薄弱标签</span>
                    <div class="assistant-tag-row">
                      <span v-for="tag in message.parsed.weak_tags" :key="`${message.id}-${tag}`" class="assistant-tag">{{ tag }}</span>
                    </div>
                  </div>

                  <div v-if="message.parsed.recommended_problems.length" class="assistant-block">
                    <span class="assistant-label">推荐题目</span>
                    <div class="recommend-list">
                      <router-link
                        v-for="problem in message.parsed.recommended_problems"
                        :key="`${message.id}-${problem.problem_id}`"
                        :to="`/problems/${problem.problem_id}`"
                        class="recommend-item"
                      >
                        <div class="recommend-item-head">
                          <span class="recommend-id">#{{ problem.problem_id }}</span>
                          <strong>{{ problem.title }}</strong>
                        </div>
                        <p>{{ problem.reason }}</p>
                        <span class="recommend-enter">查看题目</span>
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

        <div v-else-if="loadingMessages" class="thread-state">
          <span class="spinner spinner-dark"></span>
          <span>正在加载会话消息...</span>
        </div>

        <div v-else class="thread-welcome">
          <div class="thread-welcome-copy">
            <span>准备开始</span>
            <h3>像聊天一样规划你的刷题节奏</h3>
            <p>可以直接问算法问题、回忆模糊题目，或者让助手给你安排下一轮训练。</p>
          </div>

          <div class="prompt-grid">
            <button v-for="preset in promptPresets" :key="preset" class="prompt-chip" @click="draft = preset">
              {{ preset }}
            </button>
          </div>
        </div>

        <div v-if="turnPending" class="message-item message-item-assistant pending-row">
          <div class="message-avatar message-avatar-assistant">AI</div>
          <div class="message-card message-card-assistant pending-card">
            <strong>正在生成回复</strong>
            <span>{{ streamMessage || '完成后会自动写入当前会话。' }}</span>
          </div>
        </div>
      </div>

      <div class="composer-shell">
        <div v-if="messageError" class="composer-alert">{{ messageError }}</div>

        <textarea
          id="study-chat-input"
          v-model.trim="draft"
          class="composer-input"
          placeholder="输入你的问题，例如：我最近图论建模总是卡住，先帮我分析问题，再给我安排一轮练习。"
          @keydown.enter.exact.prevent="handleSendMessage"
        ></textarea>

        <div class="composer-footer">
          <span class="composer-hint">{{ sessionStatusDescription }}</span>
          <div class="composer-actions">
            <button class="sidebar-secondary" :disabled="loadingMessages || !activeSessionId" @click="reloadActiveSession">
              <span v-if="loadingMessages" class="spinner spinner-dark"></span>
              <span v-else>刷新消息</span>
            </button>
            <button class="sidebar-primary" :disabled="sending || !draft.trim()" @click="handleSendMessage">
              <span v-if="sending" class="spinner"></span>
              <span v-else>发送</span>
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createStudyPlanSession,
  deleteStudyPlanSession,
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
const deletingSession = ref(false)
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
      return 'meta-pill-live'
    case 'connecting':
    case 'reconnecting':
      return 'meta-pill-warn'
    case 'error':
      return 'meta-pill-error'
    default:
      return ''
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

    persistSessionId(activeSessionId.value)

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

async function handleDeleteSession() {
  if (!activeSessionId.value) {
    return
  }
  if (turnPending.value) {
    sessionMessage.value = '当前会话还有一条回复在生成，暂时不能删除。'
    sessionMessageType.value = 'error'
    return
  }
  if (!window.confirm('删除当前会话后，它会从列表中隐藏。确认继续吗？')) {
    return
  }

  deletingSession.value = true
  sessionMessage.value = ''
  messageError.value = ''

  try {
    const deletingSessionId = activeSessionId.value
    closeTurnStream()
    await deleteStudyPlanSession(deletingSessionId)
    messages.value = []
    currentTurn.value = null
    draft.value = ''

    await loadSessions(false)
    if (activeSessionId.value > 0) {
      await loadMessages(activeSessionId.value)
    }

    sessionMessage.value = '会话已删除。'
    sessionMessageType.value = 'success'
  } catch (error) {
    sessionMessage.value = getErrorMessage(error, '删除会话失败。')
    sessionMessageType.value = 'error'
  } finally {
    deletingSession.value = false
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
.chat-shell {
  display: grid;
  grid-template-columns: 308px minmax(0, 1fr);
  gap: 0;
  height: calc(100vh - 132px);
  max-height: calc(100vh - 132px);
  overflow: hidden;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 32px;
  background:
    radial-gradient(circle at top left, rgba(59, 130, 246, 0.08), transparent 28%),
    linear-gradient(180deg, #f7f9fc 0%, #f4f7fb 100%);
  box-shadow: 0 30px 80px rgba(15, 23, 42, 0.12);
}

.chat-sidebar {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-height: 0;
  padding: 18px 14px 18px 18px;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(12, 18, 31, 0.98) 0%, rgba(17, 24, 39, 0.97) 100%);
  color: #eef2ff;
}

.sidebar-top {
  display: grid;
  gap: 14px;
  padding: 4px 4px 16px;
}

.sidebar-brand {
  display: grid;
  gap: 8px;
}

.sidebar-kicker {
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: rgba(191, 219, 254, 0.82);
}

.sidebar-brand h1 {
  margin: 0;
  font-size: 28px;
  letter-spacing: -0.05em;
}

.sidebar-brand p {
  margin: 0;
  color: rgba(226, 232, 240, 0.7);
  line-height: 1.6;
}

.sidebar-toolbar {
  display: flex;
  gap: 10px;
}

.sidebar-primary,
.sidebar-secondary,
.danger-pill,
.prompt-chip,
.session-row,
.recommend-item {
  transition: transform var(--transition), box-shadow var(--transition), border-color var(--transition), background var(--transition), color var(--transition);
}

.sidebar-primary,
.sidebar-secondary,
.danger-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 42px;
  padding: 0 16px;
  border-radius: 999px;
  font-size: 14px;
  font-weight: 700;
}

.sidebar-primary {
  border: 1px solid rgba(59, 130, 246, 0.52);
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  color: #f8fbff;
  box-shadow: 0 16px 32px rgba(37, 99, 235, 0.22);
}

.sidebar-primary:hover:not(:disabled) {
  transform: translateY(-1px);
}

.sidebar-secondary {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.06);
  color: #e2e8f0;
}

.sidebar-secondary:hover:not(:disabled) {
  border-color: rgba(148, 163, 184, 0.48);
  background: rgba(255, 255, 255, 0.1);
}

.sidebar-alert {
  padding: 12px 14px;
  border-radius: 16px;
  font-size: 13px;
  line-height: 1.6;
}

.sidebar-alert-success {
  background: rgba(22, 163, 74, 0.16);
  color: #bbf7d0;
}

.sidebar-alert-error {
  background: rgba(220, 38, 38, 0.16);
  color: #fecaca;
}

.session-scroll {
  min-height: 0;
  overflow: auto;
  padding-right: 6px;
}

.session-stack {
  display: grid;
  gap: 10px;
  align-content: start;
}

.session-row {
  display: grid;
  gap: 8px;
  width: 100%;
  text-align: left;
  padding: 14px 14px 13px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.04);
  color: #eef2ff;
}

.session-row:hover {
  transform: translateY(-1px);
  border-color: rgba(96, 165, 250, 0.28);
  background: rgba(255, 255, 255, 0.07);
}

.session-row.active {
  border-color: rgba(96, 165, 250, 0.44);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.22), rgba(59, 130, 246, 0.14));
  box-shadow: inset 0 0 0 1px rgba(147, 197, 253, 0.1);
}

.session-row-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.session-row-head strong {
  font-size: 15px;
  line-height: 1.5;
}

.session-row-head span,
.session-row-time,
.session-row-memory {
  color: rgba(191, 219, 254, 0.72);
}

.session-row-head span,
.session-row-memory {
  font-size: 12px;
}

.session-row-time {
  font-size: 13px;
}

.session-row-memory {
  font-weight: 700;
}

.sidebar-empty {
  display: grid;
  gap: 8px;
  padding: 20px 16px;
  border: 1px dashed rgba(148, 163, 184, 0.2);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.04);
}

.sidebar-empty strong {
  color: #f8fafc;
}

.sidebar-empty p {
  margin: 0;
  color: rgba(226, 232, 240, 0.68);
  line-height: 1.6;
}

.chat-main {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  min-height: 0;
  height: 100%;
  padding: 18px 22px 20px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.74) 0%, rgba(248, 250, 252, 0.96) 100%);
  overflow: hidden;
}

.chat-main-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 4px 0 14px;
}

.chat-main-copy {
  display: grid;
  gap: 8px;
}

.chat-main-kicker {
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ink-faint);
}

.chat-main-copy h2 {
  margin: 0;
  font-size: clamp(28px, 3vw, 38px);
  letter-spacing: -0.05em;
}

.chat-main-copy p {
  margin: 0;
  color: var(--ink-soft);
}

.chat-main-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
  max-width: 420px;
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 38px;
  padding: 0 14px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.82);
  color: var(--ink-soft);
  font-size: 13px;
  font-weight: 700;
}

.meta-pill-live {
  border-color: rgba(13, 148, 136, 0.2);
  background: rgba(20, 184, 166, 0.1);
  color: #0f766e;
}

.meta-pill-warn {
  border-color: rgba(217, 119, 6, 0.2);
  background: rgba(245, 158, 11, 0.12);
  color: #b45309;
}

.meta-pill-error {
  border-color: rgba(220, 38, 38, 0.16);
  background: rgba(239, 68, 68, 0.1);
  color: #b91c1c;
}

.danger-pill {
  border: 1px solid rgba(220, 38, 38, 0.14);
  background: rgba(255, 255, 255, 0.84);
  color: #b91c1c;
}

.danger-pill:hover:not(:disabled) {
  transform: translateY(-1px);
  background: rgba(254, 242, 242, 0.96);
}

.memory-strip {
  display: grid;
  gap: 8px;
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.78);
}

.memory-strip-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.memory-strip p {
  margin: 0;
  color: var(--ink-soft);
  white-space: pre-wrap;
  line-height: 1.7;
}

.chat-thread {
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  gap: 18px;
  padding: 6px 4px 20px;
  overscroll-behavior: contain;
}

.message-item {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
  width: min(100%, 920px);
}

.message-item-user {
  justify-self: end;
  grid-template-columns: minmax(0, 1fr) 42px;
}

.message-item-user .message-avatar {
  order: 2;
}

.message-item-user .message-card {
  order: 1;
}

.message-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 800;
}

.message-avatar-user {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  color: #f8fbff;
}

.message-avatar-assistant {
  background: rgba(15, 23, 42, 0.08);
  color: var(--ink);
}

.message-card {
  display: grid;
  gap: 12px;
  padding: 16px 18px;
  border-radius: 24px;
  border: 1px solid rgba(15, 23, 42, 0.08);
}

.message-card-assistant {
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.message-card-user {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  color: #f8fbff;
  box-shadow: 0 16px 36px rgba(37, 99, 235, 0.2);
}

.message-card-user .message-head span,
.message-card-user .message-text {
  color: rgba(248, 251, 255, 0.84);
}

.message-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.message-head span {
  color: var(--ink-soft);
}

.message-content,
.assistant-block {
  display: grid;
  gap: 12px;
}

.message-text {
  white-space: pre-wrap;
  line-height: 1.8;
}

.assistant-summary {
  font-size: 15px;
}

.assistant-label {
  font-size: 12px;
  font-weight: 800;
  color: var(--ink-soft);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.assistant-tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.assistant-tag {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  background: rgba(217, 119, 6, 0.12);
  color: #b45309;
  font-size: 13px;
  font-weight: 700;
}

.recommend-list {
  display: grid;
  gap: 12px;
}

.recommend-item {
  display: grid;
  gap: 8px;
  padding: 14px 16px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 18px;
  background: rgba(245, 248, 255, 0.84);
}

.recommend-item:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.2);
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.08);
}

.recommend-item-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.recommend-id {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  background: rgba(37, 99, 235, 0.1);
  color: var(--brand-deep);
  font-size: 12px;
  font-weight: 800;
}

.recommend-item p {
  margin: 0;
  color: var(--ink-soft);
  line-height: 1.6;
}

.recommend-enter {
  color: var(--brand-deep);
  font-weight: 700;
}

.thread-state,
.thread-welcome {
  width: min(100%, 920px);
  justify-self: center;
}

.thread-state {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 18px 0;
  color: var(--ink-soft);
}

.thread-welcome {
  display: grid;
  gap: 18px;
  padding: 26px;
  border: 1px dashed rgba(15, 23, 42, 0.12);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.62);
}

.thread-welcome-copy {
  display: grid;
  gap: 8px;
}

.thread-welcome-copy span {
  font-size: 12px;
  font-weight: 800;
  color: var(--ink-faint);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.thread-welcome-copy h3 {
  margin: 0;
  font-size: 28px;
  letter-spacing: -0.04em;
}

.thread-welcome-copy p {
  margin: 0;
  color: var(--ink-soft);
  line-height: 1.7;
}

.prompt-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.prompt-chip {
  text-align: left;
  padding: 16px 18px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.88);
  color: var(--ink);
  font-weight: 600;
}

.prompt-chip:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.22);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.pending-row {
  justify-self: start;
}

.pending-card strong {
  font-size: 15px;
}

.pending-card span {
  color: var(--ink-soft);
}

.composer-shell {
  display: grid;
  gap: 12px;
  margin-top: 12px;
  padding-top: 16px;
  border-top: 1px solid rgba(15, 23, 42, 0.08);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0) 0%, rgba(248, 250, 252, 0.94) 26%);
  flex-shrink: 0;
}

.composer-alert {
  padding: 12px 14px;
  border-radius: 16px;
  background: rgba(220, 38, 38, 0.1);
  color: #b91c1c;
  font-size: 13px;
}

.composer-input {
  width: 100%;
  min-height: 112px;
  max-height: 160px;
  padding: 18px 20px;
  border: 1px solid rgba(15, 23, 42, 0.1);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.96);
  color: var(--ink);
  font: inherit;
  resize: none;
  overflow: auto;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.composer-input:focus {
  outline: none;
  border-color: rgba(37, 99, 235, 0.28);
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.08);
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.composer-hint {
  color: var(--ink-soft);
  font-size: 13px;
}

.composer-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.62;
  transform: none !important;
  box-shadow: none !important;
}

@media (max-width: 1180px) {
  .chat-shell {
    grid-template-columns: 280px minmax(0, 1fr);
  }
}

@media (max-width: 980px) {
  .chat-shell {
    grid-template-columns: 1fr;
    height: auto;
    max-height: none;
  }

  .chat-sidebar {
    border-right: 0;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }

  .session-scroll {
    max-height: 260px;
  }
}

@media (max-width: 760px) {
  .chat-shell {
    min-height: calc(100vh - 96px);
  }

  .chat-main,
  .chat-sidebar {
    padding-left: 14px;
    padding-right: 14px;
  }

  .chat-main-header,
  .composer-footer {
    display: grid;
    align-items: start;
  }

  .chat-main-meta {
    justify-content: flex-start;
  }

  .message-item,
  .message-item-user,
  .pending-row {
    grid-template-columns: 1fr;
    width: 100%;
  }

  .message-item-user .message-avatar,
  .message-item-user .message-card {
    order: initial;
  }

  .message-avatar {
    width: 38px;
    height: 38px;
  }

  .message-card {
    border-radius: 20px;
  }

  .prompt-grid {
    grid-template-columns: 1fr;
  }
}
</style>
