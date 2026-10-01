<template>
  <div class="chat-shell">
    <aside id="chat-history" class="chat-sidebar" :class="{ 'history-open': historyOpen }">
      <button class="history-close icon-button" aria-label="关闭会话列表" @click="historyOpen = false"><AppIcon name="close" /></button>
      <div class="sidebar-top">
        <div class="sidebar-brand">
          <span class="sidebar-kicker">AI Study Assistant</span>
          <h1>学习助手</h1>
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
            @click="selectSession(session.id); historyOpen = false"
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
        <button class="history-toggle icon-button" aria-label="打开会话列表" aria-controls="chat-history" :aria-expanded="historyOpen" @click="historyOpen = !historyOpen"><AppIcon name="history" /></button>
        <div class="chat-main-copy">
          <span class="chat-main-kicker">Conversation</span>
          <h2>{{ activeSession ? sessionTitle(activeSession) : '新的对话' }}</h2>
          <p>{{ sessionStatusDescription }}</p>
        </div>

        <div class="chat-main-meta">
          <span class="meta-pill" :class="streamBadgeClass">{{ streamLabel }}</span>
          <button class="danger-pill" :disabled="deletingSession || !activeSessionId || turnPending" @click="handleDeleteSession">
            <span v-if="deletingSession" class="spinner spinner-dark"></span>
            <span v-else>删除会话</span>
          </button>
        </div>
      </header>

      <div v-if="activeSession?.summary_text" class="memory-strip">
        <div class="memory-strip-head">
          <span>对话摘要</span>
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
                    {{ message.parsed.answer || message.content }}
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
                  <div v-if="message.turn_id" class="feedback-panel">
                    <span class="feedback-label">这次回答有帮助吗？</span>
                    <div class="feedback-actions">
                      <button
                        class="feedback-button"
                        :class="{ active: feedbackByTurn[message.turn_id]?.helpful === true }"
                        :disabled="feedbackSubmitting[message.turn_id]"
                        @click="submitFeedback(message.turn_id, true)"
                      >
                        有帮助
                      </button>
                      <button
                        class="feedback-button"
                        :class="{ active: feedbackByTurn[message.turn_id]?.helpful === false }"
                        :disabled="feedbackSubmitting[message.turn_id]"
                        @click="submitFeedback(message.turn_id, false)"
                      >
                        需改进
                      </button>
                    </div>
                    <input
                      v-model.trim="feedbackDrafts[message.turn_id]"
                      class="feedback-input"
                      :disabled="feedbackSubmitting[message.turn_id]"
                      placeholder="可选：补充反馈，帮助改进后续回答"
                      @keydown.enter.prevent="submitFeedback(message.turn_id, feedbackByTurn[message.turn_id]?.helpful ?? true)"
                    >
                    <small v-if="feedbackMessage[message.turn_id]" class="feedback-message">{{ feedbackMessage[message.turn_id] }}</small>
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
          <div class="welcome-symbol"><AppIcon name="sparkles" /></div>
          <div class="thread-welcome-copy">
            <span>THINK TOGETHER. GO FURTHER.</span>
            <h3>今天，想攻克什么问题？</h3>
            <p>一起拆解思路、回顾练习，找到下一步的方向。</p>
          </div>

          <div class="prompt-grid">
            <button v-for="preset in promptPresets" :key="preset" class="prompt-chip" @click="draft = preset">
              <AppIcon name="arrow" />{{ preset }}
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
          aria-label="向 AI 学习助手提问"
          v-model.trim="draft"
          class="composer-input" :disabled="sending || turnPending"
          placeholder="聊聊你遇到的算法问题，或一起规划下一轮练习…"
          @keydown.enter.exact="handleComposerEnter"
        ></textarea>

        <div class="composer-footer">
          <span class="composer-hint">Enter 发送 · Shift + Enter 换行</span>
          <div class="composer-actions">
            <button class="sidebar-secondary" :disabled="loadingMessages || !activeSessionId" @click="reloadActiveSession">
              <span v-if="loadingMessages" class="spinner spinner-dark"></span>
              <span v-else>刷新消息</span>
            </button>
            <button class="sidebar-primary" :disabled="sending || turnPending || !draft.trim()" @click="handleSendMessage">
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
function handleComposerEnter(event) {
  if (event.isComposing) return
  event.preventDefault()
  handleSendMessage()
}

import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createChatSession,
  createChatStreamTicket,
  deleteChatSession,
  getErrorMessage,
  getChatMessages,
  getChatPlanFeedback,
  getChatTurn,
  listChatSessions,
  sendChatMessage,
  submitChatPlanFeedback,
} from '../api'

const CHAT_SESSION_KEY = 'gojo:chatSessionId'
const ACTIVE_TURN_STATUSES = ['pending', 'running']
const TERMINAL_TURN_STATUSES = ['succeeded', 'failed']

const route = useRoute()
const router = useRouter()

const promptPresets = [
  '我最近动态规划总在状态设计上卡住，先帮我定位问题。',
  '准备笔试，给我安排一轮偏图论和最短路的训练。',
  '我记得有道题和饭量有关，帮我回忆一下是哪道题。',
]

const historyOpen = ref(false)
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
const feedbackByTurn = ref({})
const feedbackDrafts = ref({})
const feedbackSubmitting = ref({})
const feedbackMessage = ref({})

let turnStream = null
let turnStreamTurnId = 0
let turnStatusPollTimer = null

const activeSession = computed(() => sessions.value.find((item) => item.id === activeSessionId.value) || null)

const normalizedMessages = computed(() => messages.value.map((message) => ({
  ...message,
  parsed: parseAssistantPayload(message),
})))

const turnPending = computed(() => ACTIVE_TURN_STATUSES.includes(currentTurn.value?.status || ''))

const sessionStatusDescription = computed(() => {
  if (turnPending.value) {
    return '正在思考你的问题，回复会自动显示。'
  }
  if (activeSession.value) {
    return '沿着这个话题继续探索，你的学习记录会自动保存。'
  }
  return '从一个问题开始，让思路变得清晰。'
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
      return '准备就绪'
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
    localStorage.setItem(CHAT_SESSION_KEY, String(sessionId))
  } else {
    localStorage.removeItem(CHAT_SESSION_KEY)
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

  const storedSessionId = Number(localStorage.getItem(CHAT_SESSION_KEY) || 0)
  return storedSessionId > 0 ? storedSessionId : 0
}

function closeTurnStream(resetState = true) {
  if (turnStream) {
    turnStream.close()
    turnStream = null
  }
  turnStreamTurnId = 0

  if (turnStatusPollTimer) {
    window.clearInterval(turnStatusPollTimer)
    turnStatusPollTimer = null
  }

  if (resetState) {
    streamState.value = 'idle'
    streamMessage.value = ''
  }
}

async function refreshTurnStatus(turnId, source = null) {
  if (!turnId || (source && turnStream !== source)) {
    return false
  }

  try {
    const latestTurn = await getChatTurn(turnId)
    if (source && turnStream !== source) {
      return false
    }
    currentTurn.value = latestTurn

    if (!TERMINAL_TURN_STATUSES.includes(latestTurn.status || '')) {
      return false
    }

    closeTurnStream(false)
    streamState.value = 'idle'
    streamMessage.value = latestTurn.status === 'succeeded' ? '\u56de\u590d\u5df2\u751f\u6210\u5b8c\u6210\u3002' : '\u8fd9\u6b21\u56de\u590d\u751f\u6210\u5931\u8d25\uff0c\u8bf7\u91cd\u8bd5\u3002'
    await loadSessions(false, activeSessionId.value)
    await loadMessages(activeSessionId.value)
    return true
  } catch {
    // Keep the existing state while a transient status request is retried.
    return false
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
      answer: parsed?.answer || '',
    }
  } catch {
    return null
  }
}


function assistantTurnIds(items) {
  return [...new Set(items
    .filter((item) => item.role === 'assistant' && Number(item.turn_id || 0) > 0)
    .map((item) => Number(item.turn_id)))]
}

async function loadFeedbackForMessages(items) {
  feedbackByTurn.value = {}
  feedbackDrafts.value = {}
  feedbackMessage.value = {}

  await Promise.all(assistantTurnIds(items).map(async (turnId) => {
    try {
      const feedback = await getChatPlanFeedback(turnId)
      feedbackByTurn.value[turnId] = feedback
      feedbackDrafts.value[turnId] = feedback.comment || ''
    } catch {
      // A missing feedback record is the normal first-use state.
    }
  }))
}

async function submitFeedback(turnId, helpful) {
  if (!turnId || feedbackSubmitting.value[turnId]) {
    return
  }

  feedbackSubmitting.value[turnId] = true
  feedbackMessage.value[turnId] = ''
  try {
    const feedback = await submitChatPlanFeedback(turnId, {
      helpful,
      comment: feedbackDrafts.value[turnId] || '',
    })
    feedbackByTurn.value[turnId] = feedback
    feedbackDrafts.value[turnId] = feedback.comment || ''
    feedbackMessage.value[turnId] = '反馈已保存。'
  } catch (error) {
    feedbackMessage.value[turnId] = getErrorMessage(error, '保存反馈失败，请稍后重试。')
  } finally {
    feedbackSubmitting.value[turnId] = false
  }
}
async function ensureActiveSession() {
  if (activeSessionId.value > 0) {
    return activeSessionId.value
  }

  const created = await createChatSession({ title: '' })
  await loadSessions(false, created.id)
  persistSessionId(created.id)
  return created.id
}

async function loadSessions(showMessage = false, preferredSessionId = 0) {
  loadingSessions.value = true

  try {
    const items = await listChatSessions({ limit: 50 })
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
    const turn = await getChatTurn(lastTurnId)
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
    const items = await getChatMessages(sessionId)
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

function startTurnStatusPolling(turnId) {
  if (turnStatusPollTimer) {
    window.clearInterval(turnStatusPollTimer)
  }
  turnStatusPollTimer = window.setInterval(() => {
    void refreshTurnStatus(turnId)
  }, 2000)
}

async function connectTurnStream(turnId) {
  if (!turnId || (turnStream && turnStreamTurnId === turnId)) {
    return
  }

  closeTurnStream(false)
  streamState.value = 'connecting'
  streamMessage.value = '正在建立回复同步连接...'

  let ticket = ''
  try {
    ticket = await createChatStreamTicket(turnId)
  } catch {
    streamState.value = 'error'
    streamMessage.value = '实时连接建立失败，已切换为状态轮询。'
    startTurnStatusPolling(turnId)
    return
  }

  if (!ticket) {
    streamState.value = 'error'
    streamMessage.value = '实时连接凭证无效，已切换为状态轮询。'
    startTurnStatusPolling(turnId)
    return
  }

  const source = new EventSource(`/api/chat/turns/${turnId}/stream?ticket=${encodeURIComponent(ticket)}`)
  turnStream = source
  turnStreamTurnId = turnId
  startTurnStatusPolling(turnId)

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
        await refreshTurnStatus(turnId, source)
      }
    } catch {
      closeTurnStream(false)
      streamState.value = 'error'
      streamMessage.value = '实时数据解析失败，请刷新当前会话。'
    }
  }

  source.onerror = async () => {
    if (turnStream !== source) {
      return
    }

    const reachedTerminalState = await refreshTurnStatus(turnId, source)
    if (reachedTerminalState || turnStream !== source) {
      return
    }

    // A ticket is one-time, so EventSource cannot reuse it for browser-level
    // reconnects. Close it and retain the existing status-polling fallback.
    closeTurnStream(false)
    streamState.value = 'error'
    streamMessage.value = '实时连接中断，已切换为状态轮询。'
    startTurnStatusPolling(turnId)
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
    await deleteChatSession(deletingSessionId)
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
    const created = await createChatSession({ title: '' })
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
  if (sending.value) {
    return
  }
  if (turnPending.value) {
    messageError.value = '当前回复尚未完成，请等待后再继续发送。'
    return
  }

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
    const createdTurn = await sendChatMessage(sessionId, { content })
    draft.value = ''
    currentTurn.value = {
      id: createdTurn.turn_id,
      status: createdTurn.status,
      model: createdTurn.model,
      session_id: createdTurn.session_id,
    }
    await loadSessions(false, sessionId)
    await loadMessages(sessionId)
    if (currentTurn.value?.id === createdTurn.turn_id && turnPending.value) {
      connectTurnStream(createdTurn.turn_id)
    }
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
  grid-template-columns: 260px minmax(0, 1fr);
  height: calc(100dvh - 138px);
  min-height: 620px;
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 16px;
  background: white;
  box-shadow: var(--shadow-md);
  position: relative;
}

.chat-sidebar {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 24px 16px;
  background: #142440;
  color: #d7e3f7;
  position: relative;
}

.sidebar-top {
  display: grid;
  gap: 20px;
  padding: 0 4px 20px;
}

.sidebar-brand {
  display: grid;
  gap: 8px;
}

.sidebar-kicker {
  font-size: 8px;
  font-weight: 600;
  letter-spacing: 1.8px;
  color: #869fc5;
}

.sidebar-brand h1 {
  font-size: 24px;
  margin: 0;
  font-weight: 600;
}

.sidebar-brand p {
  font-size: 11px;
  line-height: 1.8;
  margin: 0;
  color: #859cbc;
}

.sidebar-toolbar {
  display: flex;
  gap: 8px;
}

.sidebar-primary, .sidebar-secondary, .danger-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding: 8px 12px;
  border-radius: 7px;
  font-size: 11px;
  font-weight: 600;
  gap: 7px;
  transition: background var(--transition);
}

.sidebar-primary {
  color: white;
  background: var(--brand);
}

.sidebar-primary:hover:not(:disabled) {
  background: var(--brand-deep);
}

.sidebar-toolbar .sidebar-primary {
  flex: 1;
}

.sidebar-secondary {
  background: #ffffff08;
  color: #b5c9e7;
  border: 1px solid #8ca8d52b;
}

.sidebar-secondary:hover:not(:disabled) {
  background: #ffffff12;
}

.sidebar-alert {
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 1.7;
}

.sidebar-alert-success {
  background: #6fc69718;
  color: #b3efcd;
}

.sidebar-alert-error {
  background: #ff879018;
  color: #ffb8b8;
}

.session-scroll {
  overflow: auto;
  min-height: 0;
  flex: 1;
}

.session-stack {
  display: grid;
  gap: 6px;
  align-content: start;
}

.session-row {
  display: grid;
  gap: 6px;
  width: 100%;
  text-align: left;
  padding: 14px 12px;
  border-radius: 8px;
  border: 1px solid transparent;
  color: #b5c9e7;
}

.session-row:hover {
  background: #ffffff06;
}

.session-row.active {
  background: #ffffff08;
  border-color: #6382b040;
  color: white;
}

.session-row-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.session-row-head strong {
  font-size: 12px;
  font-weight: 550;
  overflow-wrap: anywhere;
}

.session-row-head span {
  font: 9px Consolas, monospace;
  color: #6781a7;
}

.session-row-time {
  font-size: 9px;
  color: #6f89af;
}

.session-row-memory {
  font-size: 9px;
  color: #84b7a3;
}

.sidebar-empty {
  padding: 22px 14px;
  border: 1px dashed #8ca8d52b;
  border-radius: 8px;
}

.sidebar-empty strong {
  font-size: 12px;
  font-weight: 550;
}

.sidebar-empty p {
  color: #859cbc;
  font-size: 11px;
  margin-bottom: 0;
  line-height: 1.8;
}

.chat-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 24px 30px 20px;
  background: white;
  overflow: hidden;
}

.chat-main-header {
  display: flex;
  flex-shrink: 0;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--line);
}

.chat-main-copy {
  min-width: 0;
}

.chat-main-kicker {
  color: #9ba6b9;
  font-size: 8px;
  letter-spacing: 2px;
}

.chat-main-copy h2 {
  margin: 6px 0;
  font-size: 19px;
  font-weight: 650;
  overflow-wrap: anywhere;
}

.chat-main-copy p {
  margin: 0;
  font-size: 11px;
  color: var(--ink-faint);
}

.chat-main-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
  flex-shrink: 0;
}

.meta-pill {
  padding: 5px 9px;
  border-radius: 5px;
  background: #f5f7fb;
  color: #8c99ae;
  font-size: 9px;
  display: inline-flex;
  align-items: center;
}

.meta-pill-live {
  background: #e7f6ef;
  color: var(--success);
}

.meta-pill-warn {
  background: #fff6e6;
  color: var(--warning);
}

.meta-pill-error {
  background: #fdeef0;
  color: var(--danger);
}

.danger-pill {
  color: #b47c85;
  border: 1px solid var(--line);
  min-height: 30px;
  padding: 5px 9px;
  font-size: 9px;
}

.danger-pill:hover:not(:disabled) {
  color: var(--danger);
  background: #fdeef0;
}

.memory-strip {
  margin-top: 16px;
  padding: 12px 16px;
  border: 1px solid #e4eaf7;
  border-radius: 8px;
  background: #f7f9fe;
  flex-shrink: 0;
  max-height: 130px;
  overflow-y: auto;
}

.memory-strip-head {
  display: flex;
  justify-content: space-between;
  font-size: 10px;
  color: #7e8da9;
}

.memory-strip p {
  margin: 6px 0 0;
  white-space: pre-wrap;
  color: var(--ink-soft);
  font-size: 11px;
  line-height: 1.8;
}

.chat-thread {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  gap: 24px;
  padding: 24px 4px;
  overscroll-behavior: contain;
}

.message-item {
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr);
  gap: 12px;
  width: min(100%, 840px);
  align-items: start;
}

.message-item-user {
  grid-template-columns: minmax(0, 1fr) 32px;
  justify-self: end;
  width: min(90%, 760px);
}

.message-item-user .message-avatar {
  order: 2;
}

.message-item-user .message-card {
  order: 1;
}

.message-avatar {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 9px;
  font-size: 10px;
  font-weight: 650;
}

.message-avatar-user {
  background: var(--brand);
  color: white;
}

.message-avatar-assistant {
  background: #e9f4ed;
  color: #438665;
}

.message-card {
  padding: 14px 18px;
  border-radius: 10px;
  display: grid;
  gap: 12px;
  min-width: 0;
}

.message-card-assistant {
  border: 1px solid var(--line);
  background: white;
}

.message-card-user {
  border: 1px solid #e1e8fd;
  background: #f2f5ff;
}

.message-head {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 6px;
  font-size: 11px;
}

.message-head strong {
  font-weight: 650;
}

.message-head span {
  font-size: 9px;
  color: #9aa6ba;
}

.message-content, .assistant-block {
  display: grid;
  gap: 12px;
}

.message-text {
  white-space: pre-wrap;
  line-height: 1.9;
  overflow-wrap: anywhere;
  font-size: 13px;
}

.assistant-label {
  font-size: 10px;
  font-weight: 650;
  color: #7a89a0;
}

.assistant-tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.assistant-tag {
  font-size: 10px;
  padding: 3px 8px;
  background: #fff6e6;
  color: #b28b46;
  border-radius: 5px;
}

.recommend-list {
  display: grid;
  gap: 10px;
}

.recommend-item {
  display: grid;
  gap: 6px;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 12px;
  background: #fafbfe;
}

.recommend-item:hover {
  border-color: #315ee740;
}

.recommend-item-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.recommend-id {
  font: 10px Consolas, monospace;
  color: var(--brand);
  background: var(--surface-tint);
  padding: 3px 6px;
  border-radius: 4px;
}

.recommend-item p {
  margin: 0;
  font-size: 11px;
  color: var(--ink-faint);
}

.recommend-enter {
  font-size: 10px;
  color: var(--brand);
}

.thread-state {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--ink-faint);
}

.thread-welcome {
  justify-self: center;
  align-self: center;
  display: grid;
  justify-items: center;
  gap: 22px;
  width: min(100%, 640px);
  padding: 38px 12px 24px;
}

.welcome-symbol {
  width: 60px;
  height: 60px;
  display: grid;
  place-items: center;
  color: var(--brand);
  background: #f0f4ff;
  border: 1px solid #e6ebfb;
  border-radius: 18px;
}

.welcome-symbol .app-icon {
  width: 30px;
  height: 30px;
}

.thread-welcome-copy {
  text-align: center;
}

.thread-welcome-copy span {
  font-size: 8px;
  letter-spacing: 2px;
  color: #9eaac0;
}

.thread-welcome-copy h3 {
  margin: 12px 0;
  font-size: 26px;
  font-weight: 650;
}

.thread-welcome-copy p {
  font-size: 12px;
  color: var(--ink-faint);
  margin: 0;
}

.prompt-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  width: 100%;
  gap: 10px;
}

.prompt-chip {
  display: grid;
  gap: 12px;
  text-align: left;
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #fafbfe;
  font-size: 11px;
  line-height: 1.8;
  color: #6b7b97;
}

.prompt-chip .app-icon {
  width: 16px;
  height: 16px;
  color: #98aace;
}

.prompt-chip:hover {
  border-color: #315ee740;
  background: #f5f7ff;
  color: var(--brand);
}

.pending-card {
  border-color: #e4eaf7;
}

.pending-card strong {
  font-size: 12px;
  color: var(--brand);
}

.pending-card span {
  font-size: 11px;
  color: var(--ink-faint);
}

.composer-shell {
  flex-shrink: 0;
  display: grid;
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid var(--line);
}

.composer-alert {
  padding: 10px 12px;
  border-radius: 8px;
  background: #fdeef0;
  color: var(--danger);
  font-size: 11px;
}

.composer-input {
  width: 100%;
  min-height: 94px;
  max-height: 160px;
  padding: 16px;
  border: 1px solid var(--line-strong);
  border-radius: 10px;
  color: var(--ink);
  background: #fafbfe;
  resize: none;
  outline: none;
  font-size: 12px;
  line-height: 1.8;
}

.composer-input:focus {
  border-color: var(--brand);
  box-shadow: 0 0 0 3px #315ee714;
}

.composer-footer {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: center;
}

.composer-hint {
  font-size: 9px;
  color: #9aa6ba;
}

.composer-actions {
  display: flex;
  gap: 8px;
}

.composer-actions .sidebar-secondary {
  color: var(--ink-faint);
  background: white;
  border-color: var(--line);
}

.composer-actions .sidebar-primary {
  padding-inline: 20px;
}

.feedback-panel {
  display: grid;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--line);
}

.feedback-label, .feedback-message {
  font-size: 10px;
  color: var(--ink-faint);
}

.feedback-actions {
  display: flex;
  gap: 8px;
}

.feedback-button {
  border: 1px solid var(--line);
  padding: 4px 10px;
  border-radius: 5px;
  font-size: 10px;
  color: var(--ink-soft);
}

.feedback-button.active {
  background: var(--accent-soft);
  border-color: #168b7730;
  color: var(--accent);
}

.feedback-input {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  font-size: 11px;
  outline: none;
}

.feedback-input:focus {
  border-color: var(--brand);
}

.history-toggle, .history-close {
  display: none;
}

@media (max-width: 1080px) {
  .chat-shell {
    grid-template-columns: 224px minmax(0, 1fr);
  }
  .chat-main {
    padding: 20px;
  }
  .prompt-grid {
    grid-template-columns: 1fr;
  }
  .prompt-chip {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px;
  }
  .prompt-chip .app-icon {
    margin-top: 3px;
  }
  .thread-welcome {
    padding: 26px 12px;
  }
  .chat-main-copy p {
    display: none;
  }
}

@media (max-width: 760px) {
  .composer-input {
    font-size: 16px;
  }
  .chat-shell {
    grid-template-columns: minmax(0, 1fr);
    height: calc(100dvh - 108px);
    min-height: 580px;
  }
  .chat-sidebar {
    display: none;
  }
  .chat-sidebar.history-open {
    display: flex;
    position: absolute;
    inset: 0 auto 0 0;
    width: min(300px, 90%);
    z-index: 10;
    box-shadow: 20px 0 50px #14244045;
  }
  .history-toggle {
    display: inline-flex;
    flex-shrink: 0;
  }
  .history-close {
    display: inline-flex;
    position: absolute;
    right: 14px;
    top: 14px;
    color: #a9bddc;
  }
  .chat-main {
    padding: 16px;
  }
  .chat-main-header {
    gap: 8px;
    padding-bottom: 14px;
    align-items: center;
  }
  .chat-main-copy {
    flex: 1;
  }
  .chat-main-copy h2 {
    font-size: 16px;
  }
  .chat-main-meta {
    flex-direction: column;
    gap: 4px;
  }
  .meta-pill {
    font-size: 8px;
  }
  .danger-pill {
    font-size: 8px;
    min-height: 24px;
  }
  .chat-main-kicker {
    font-size: 7px;
  }
  .thread-welcome {
    padding: 24px 0 12px;
    gap: 18px;
  }
  .thread-welcome-copy h3 {
    font-size: 22px;
  }
  .thread-welcome-copy p {
    font-size: 11px;
  }
  .composer-hint {
    font-size: 8px;
  }
  .composer-actions .sidebar-secondary {
    display: none;
  }
  .message-item, .message-item-user {
    grid-template-columns: 24px minmax(0, 1fr);
    gap: 8px;
    width: 100%;
  }
  .message-item-user {
    grid-template-columns: minmax(0, 1fr) 24px;
  }
  .message-avatar {
    width: 24px;
    height: 24px;
    border-radius: 7px;
    font-size: 8px;
  }
  .message-card {
    padding: 12px;
  }
  .message-text {
    font-size: 12px;
  }
}
</style>
