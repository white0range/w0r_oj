<template>
  <div class="page">
    <section class="page-hero study-hero">
      <div class="study-hero-copy">
        <span class="eyebrow">AI Study Plan</span>
        <div class="page-title">
          <div>
            <h1>让推荐 Agent 帮你制定下一轮刷题计划</h1>
            <p class="page-subtitle">
              这个页面直接接入后端 `study_plan` 异步任务链路。你给出目标后，系统会结合你的 AC 记录、失败提交和标签统计，
              生成一份更像专业 OJ 平台的个性化训练建议。
            </p>
          </div>
        </div>
        <div class="cluster">
          <button
            v-for="preset in goalPresets"
            :key="preset"
            class="tag-toggle"
            :class="{ active: goal.trim() === preset }"
            @click="goal = preset"
          >
            {{ preset }}
          </button>
        </div>
      </div>

      <aside class="hero-side-panel">
        <div class="hero-side-block stack">
          <div>
            <span class="meta-label">Task Status</span>
            <strong>{{ statusLabel }}</strong>
          </div>
          <p>{{ statusDescription }}</p>
          <div class="cluster">
            <span v-if="task?.model" class="pill">{{ task.model }}</span>
            <span v-if="task?.id" class="pill">Task #{{ task.id }}</span>
            <span class="pill" :class="streamBadgeClass">{{ streamLabel }}</span>
          </div>
          <p v-if="streamMessage" class="muted">{{ streamMessage }}</p>
        </div>
      </aside>
    </section>

    <section class="detail-grid">
      <article class="card stack">
        <div class="section-title">
          <h2>创建训练计划</h2>
          <span class="muted">POST /api/study-plan/tasks</span>
        </div>

        <div class="field">
          <label for="goal">这轮你想重点提升什么？</label>
          <textarea
            id="goal"
            v-model.trim="goal"
            class="textarea"
            placeholder="例如：准备秋招面试，想重点补动态规划、图论和代码实现稳定性。"
          ></textarea>
        </div>

        <div v-if="formMessage" class="auth-message" :class="formMessageType === 'error' ? 'auth-error' : 'auth-success'">
          {{ formMessage }}
        </div>

        <div class="cluster">
          <button class="btn btn-primary" :disabled="creating" @click="handleCreateTask">
            <span v-if="creating" class="spinner"></span>
            <span v-else>生成训练计划</span>
          </button>
          <button class="btn btn-outline" :disabled="loadingTask" @click="refreshTask()">
            <span v-if="loadingTask" class="spinner spinner-dark"></span>
            <span v-else>手动刷新</span>
          </button>
          <button v-if="task?.id" class="btn btn-ghost" @click="clearCurrentTask">清除当前任务</button>
        </div>
      </article>

      <article class="card stack">
        <div class="section-title">
          <h2>当前任务</h2>
          <span class="muted">GET /api/study-plan/tasks/:id + SSE</span>
        </div>

        <div v-if="task" class="task-summary-grid">
          <div class="summary-item">
            <span class="summary-label">任务编号</span>
            <strong>#{{ task.id }}</strong>
          </div>
          <div class="summary-item">
            <span class="summary-label">目标</span>
            <strong>{{ task.goal || '未填写具体目标' }}</strong>
          </div>
          <div class="summary-item">
            <span class="summary-label">状态</span>
            <strong :class="statusClass">{{ statusLabel }}</strong>
          </div>
          <div class="summary-item">
            <span class="summary-label">最近更新时间</span>
            <strong>{{ formatDate(task.updated_at || task.created_at) }}</strong>
          </div>
        </div>

        <div v-else class="empty-state compact-state">
          <strong>还没有训练任务</strong>
          <span class="muted">先提交一个目标，这里就会显示任务状态和推荐结果。</span>
        </div>

        <div v-if="task?.status === 'failed'" class="auth-message auth-error">
          {{ task.error_message || 'Agent 执行失败，请稍后重试。' }}
        </div>
      </article>
    </section>

    <section v-if="task?.status === 'running' || task?.status === 'pending'" class="loading-state">
      <strong>Agent 正在分析你的刷题画像</strong>
      <span class="muted">
        现在页面通过 SSE 接收实时任务状态；Go worker 完成任务后，结果会自动推送回来。
      </span>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-if="parsedResult">
      <section class="metric-grid">
        <article class="metric-card">
          <span class="metric-value">{{ parsedResult.weak_tags.length }}</span>
          <span class="metric-label">识别出的薄弱标签</span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ parsedResult.recommended_problems.length }}</span>
          <span class="metric-label">推荐题目数量</span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ feedback ? '已提交' : '待反馈' }}</span>
          <span class="metric-label">推荐反馈状态</span>
        </article>
      </section>

      <section class="card stack">
        <div class="section-title">
          <h2>总结建议</h2>
          <span class="muted">study_plan_summary</span>
        </div>
        <div class="summary-panel">
          {{ parsedResult.study_plan_summary || 'Agent 已完成任务，但还没有返回总结内容。' }}
        </div>
      </section>

      <section class="card stack">
        <div class="section-title">
          <h2>薄弱标签</h2>
          <span class="muted">weak_tags</span>
        </div>
        <div v-if="parsedResult.weak_tags.length" class="cluster">
          <span v-for="tag in parsedResult.weak_tags" :key="tag" class="pill weak-pill">{{ tag }}</span>
        </div>
        <div v-else class="empty-state compact-state">
          <strong>这次没有明确识别出薄弱标签</strong>
          <span class="muted">可能是历史数据还不够，或者你的目标描述还比较宽泛。</span>
        </div>
      </section>

      <section class="card stack">
        <div class="section-title">
          <h2>推荐题单</h2>
          <span class="muted">recommended_problems</span>
        </div>

        <div v-if="parsedResult.recommended_problems.length" class="recommend-grid">
          <router-link
            v-for="problem in parsedResult.recommended_problems"
            :key="problem.problem_id"
            :to="`/problems/${problem.problem_id}`"
            class="recommend-card"
          >
            <span class="mini-tag">#{{ problem.problem_id }}</span>
            <strong>{{ problem.title }}</strong>
            <p>{{ problem.reason }}</p>
            <span class="recommend-link">进入题目详情</span>
          </router-link>
        </div>

        <div v-else class="empty-state compact-state">
          <strong>这次没有生成具体推荐题目</strong>
          <span class="muted">可以换一个更具体的目标，或者先积累更多提交记录后再试一次。</span>
        </div>
      </section>

      <section class="card stack">
        <div class="section-title">
          <h2>推荐反馈</h2>
          <span class="muted">POST /api/study-plan/tasks/:id/feedback</span>
        </div>

        <div v-if="feedback" class="feedback-saved">
          <span class="badge" :class="feedback.helpful ? 'badge-success' : 'badge-warning'">
            {{ feedback.helpful ? '认为有帮助' : '认为仍需改进' }}
          </span>
          <p>{{ feedback.comment || '没有填写额外说明。' }}</p>
        </div>

        <template v-else>
          <div class="cluster">
            <button class="tag-toggle" :class="{ active: helpful === true }" @click="helpful = true">有帮助</button>
            <button class="tag-toggle" :class="{ active: helpful === false }" @click="helpful = false">还需改进</button>
          </div>

          <div class="field">
            <label for="feedback-comment">补充说明</label>
            <textarea
              id="feedback-comment"
              v-model.trim="feedbackComment"
              class="textarea"
              placeholder="例如：推荐方向挺准，但希望多给一些图论基础题。"
            ></textarea>
          </div>

          <div v-if="feedbackMessage" class="auth-message" :class="feedbackMessageType === 'error' ? 'auth-error' : 'auth-success'">
            {{ feedbackMessage }}
          </div>

          <button class="btn btn-secondary" :disabled="submittingFeedback || helpful === null || !task?.id" @click="handleSubmitFeedback">
            <span v-if="submittingFeedback" class="spinner"></span>
            <span v-else>提交反馈</span>
          </button>
        </template>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createStudyPlanTask,
  getErrorMessage,
  getStudyPlanFeedback,
  getStudyPlanTask,
  submitStudyPlanFeedback,
} from '../api'
import { store } from '../store'

const STUDY_PLAN_TASK_KEY = 'gojo:lastStudyPlanTaskId'
const ACTIVE_TASK_STATUSES = ['pending', 'running']
const TERMINAL_TASK_STATUSES = ['succeeded', 'failed']

const route = useRoute()
const router = useRouter()

const goalPresets = [
  '准备秋招面试，重点补动态规划和图论。',
  '最近总在边界处理出错，想提升代码稳定性。',
  '想为两周后的笔试做一轮高频算法冲刺。',
]

const creating = ref(false)
const loadingTask = ref(false)
const goal = ref('')
const task = ref(null)
const formMessage = ref('')
const formMessageType = ref('success')
const feedbackMessage = ref('')
const feedbackMessageType = ref('success')
const submittingFeedback = ref(false)
const helpful = ref(null)
const feedbackComment = ref('')
const feedback = ref(null)
const streamState = ref('idle')
const streamMessage = ref('')

let taskStream = null
let taskStreamTaskId = 0

const parsedResult = computed(() => {
  const raw = task.value?.result
  if (!raw) {
    return null
  }

  try {
    const data = typeof raw === 'string' ? JSON.parse(raw) : raw
    return {
      weak_tags: Array.isArray(data?.weak_tags) ? data.weak_tags : [],
      recommended_problems: Array.isArray(data?.recommended_problems) ? data.recommended_problems : [],
      study_plan_summary: data?.study_plan_summary || '',
    }
  } catch {
    return {
      weak_tags: [],
      recommended_problems: [],
      study_plan_summary: String(raw),
    }
  }
})

const statusLabel = computed(() => {
  switch (task.value?.status) {
    case 'pending':
      return '等待排队'
    case 'running':
      return 'Agent 运行中'
    case 'succeeded':
      return '推荐已生成'
    case 'failed':
      return '任务失败'
    default:
      return '尚未创建任务'
  }
})

const statusDescription = computed(() => {
  switch (task.value?.status) {
    case 'pending':
      return '任务已经进入队列，等待 worker 拉起处理。'
    case 'running':
      return 'Go worker 正在调用 Python agent 生成训练计划。'
    case 'succeeded':
      return '结果已经回写数据库，你现在可以查看总结、题单并提交反馈。'
    case 'failed':
      return '这次执行没有成功，通常与 agent 服务、依赖接口或模型请求有关。'
    default:
      return '提交目标后，这里会展示任务状态和推荐结果。'
  }
})

const statusClass = computed(() => {
  switch (task.value?.status) {
    case 'succeeded':
      return 'status-AC'
    case 'failed':
      return 'status-WA'
    case 'running':
    case 'pending':
      return 'status-Pending'
    default:
      return ''
  }
})

const streamLabel = computed(() => {
  switch (streamState.value) {
    case 'connecting':
      return 'SSE 连接中'
    case 'connected':
      return 'SSE 已连接'
    case 'reconnecting':
      return 'SSE 重连中'
    case 'error':
      return 'SSE 异常'
    default:
      return 'SSE 空闲'
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

function persistTaskId(taskId) {
  localStorage.setItem(STUDY_PLAN_TASK_KEY, String(taskId))
  router.replace({
    query: {
      ...route.query,
      task: String(taskId),
    },
  })
}

function getCurrentTaskId() {
  const queryTaskId = Number(route.query.task || 0)
  if (queryTaskId > 0) {
    return queryTaskId
  }

  const storedTaskId = Number(localStorage.getItem(STUDY_PLAN_TASK_KEY) || 0)
  return storedTaskId > 0 ? storedTaskId : 0
}

function closeTaskStream(resetState = true) {
  if (taskStream) {
    taskStream.close()
    taskStream = null
  }

  taskStreamTaskId = 0

  if (resetState) {
    streamState.value = 'idle'
    streamMessage.value = ''
  }
}

function syncTaskStream() {
  if (!task.value?.id || !ACTIVE_TASK_STATUSES.includes(task.value.status)) {
    closeTaskStream(false)
    return
  }

  connectTaskStream(task.value.id)
}

function connectTaskStream(taskId) {
  if (!taskId || (taskStream && taskStreamTaskId === taskId)) {
    return
  }

  const token = store.token
  if (!token) {
    streamState.value = 'error'
    streamMessage.value = '缺少登录凭证，无法建立实时连接。'
    return
  }

  closeTaskStream(false)

  const source = new EventSource(`/api/study-plan/tasks/${taskId}/stream?token=${encodeURIComponent(token)}`)
  taskStream = source
  taskStreamTaskId = taskId
  streamState.value = 'connecting'
  streamMessage.value = '正在建立实时连接...'

  source.onopen = () => {
    if (taskStream !== source) {
      return
    }

    streamState.value = 'connected'
    streamMessage.value = '任务状态将实时推送到当前页面。'
  }

  source.onmessage = async (event) => {
    if (taskStream !== source) {
      return
    }

    try {
      const nextTask = JSON.parse(event.data)
      task.value = nextTask
      persistTaskId(nextTask.id)
      formMessage.value = ''

      if (nextTask.status === 'succeeded') {
        await loadFeedback(nextTask.id)
        closeTaskStream(false)
        streamState.value = 'idle'
        streamMessage.value = '推荐结果已推送完成。'
        return
      }

      if (nextTask.status === 'failed') {
        closeTaskStream(false)
        streamState.value = 'idle'
        streamMessage.value = '任务已结束，可查看失败信息。'
      }
    } catch {
      streamState.value = 'error'
      streamMessage.value = '实时数据解析失败，请手动刷新。'
      closeTaskStream(false)
    }
  }

  source.onerror = () => {
    if (taskStream !== source) {
      return
    }

    if (task.value?.status && TERMINAL_TASK_STATUSES.includes(task.value.status)) {
      closeTaskStream(false)
      streamState.value = 'idle'
      return
    }

    if (source.readyState === EventSource.CLOSED) {
      closeTaskStream(false)
      streamState.value = 'error'
      streamMessage.value = '实时连接已关闭，请手动刷新一次。'
      return
    }

    streamState.value = 'reconnecting'
    streamMessage.value = '实时连接短暂中断，正在自动重连...'
  }
}

async function loadFeedback(taskId) {
  if (!taskId) {
    feedback.value = null
    return
  }

  try {
    feedback.value = await getStudyPlanFeedback(taskId)
  } catch (error) {
    if (error?.response?.status === 404) {
      feedback.value = null
      return
    }

    throw error
  }
}

async function refreshTask(taskId = getCurrentTaskId()) {
  if (!taskId) {
    return
  }

  loadingTask.value = true

  try {
    const nextTask = await getStudyPlanTask(taskId)
    task.value = nextTask
    persistTaskId(nextTask.id)
    formMessage.value = ''

    if (nextTask.status === 'succeeded') {
      await loadFeedback(nextTask.id)
      streamState.value = 'idle'
      streamMessage.value = '推荐结果已就绪。'
    } else if (nextTask.status === 'failed') {
      streamState.value = 'idle'
      streamMessage.value = '任务执行失败，请查看错误信息。'
    }

    syncTaskStream()
  } catch (error) {
    formMessage.value = getErrorMessage(error, '读取训练计划任务失败。')
    formMessageType.value = 'error'
  } finally {
    loadingTask.value = false
  }
}

function clearCurrentTask() {
  closeTaskStream()
  task.value = null
  feedback.value = null
  feedbackComment.value = ''
  helpful.value = null
  formMessage.value = ''
  feedbackMessage.value = ''
  localStorage.removeItem(STUDY_PLAN_TASK_KEY)

  const nextQuery = { ...route.query }
  delete nextQuery.task
  router.replace({ query: nextQuery })
}

async function handleCreateTask() {
  if (!goal.value.trim()) {
    formMessage.value = '先写一个明确目标，Agent 才能给出更有针对性的建议。'
    formMessageType.value = 'error'
    return
  }

  creating.value = true
  formMessage.value = ''
  feedbackMessage.value = ''
  feedback.value = null

  try {
    const created = await createStudyPlanTask({ goal: goal.value })
    formMessage.value = '训练任务已创建，正在为你生成计划。'
    formMessageType.value = 'success'
    persistTaskId(created.taskId)
    await refreshTask(created.taskId)
  } catch (error) {
    formMessage.value = getErrorMessage(error, '创建训练计划失败。')
    formMessageType.value = 'error'
  } finally {
    creating.value = false
  }
}

async function handleSubmitFeedback() {
  if (!task.value?.id || helpful.value === null) {
    return
  }

  submittingFeedback.value = true
  feedbackMessage.value = ''

  try {
    feedback.value = await submitStudyPlanFeedback(task.value.id, {
      helpful: helpful.value,
      comment: feedbackComment.value,
    })
    feedbackMessage.value = '反馈已提交，这能帮助你把这条推荐链路做成完整闭环。'
    feedbackMessageType.value = 'success'
  } catch (error) {
    feedbackMessage.value = getErrorMessage(error, '提交反馈失败。')
    feedbackMessageType.value = 'error'
  } finally {
    submittingFeedback.value = false
  }
}

onMounted(async () => {
  const taskId = getCurrentTaskId()
  if (taskId) {
    await refreshTask(taskId)
  }
})

onUnmounted(() => {
  closeTaskStream()
})
</script>

<style scoped>
.study-hero {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 20px;
}

.study-hero-copy {
  display: grid;
  gap: 18px;
}

.detail-grid {
  display: grid;
  grid-template-columns: 1.02fr 0.98fr;
  gap: 18px;
}

.task-summary-grid {
  display: grid;
  gap: 14px;
}

.summary-item {
  display: grid;
  gap: 6px;
  padding: 16px 18px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.66);
}

.summary-label {
  font-size: 12px;
  font-weight: 800;
  color: var(--ink-faint);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.summary-item strong {
  font-size: 16px;
  letter-spacing: -0.02em;
}

.summary-panel {
  padding: 20px;
  border-radius: 22px;
  background: rgba(37, 99, 235, 0.06);
  border: 1px solid rgba(37, 99, 235, 0.12);
  white-space: pre-wrap;
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

.recommend-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.recommend-card {
  display: grid;
  gap: 10px;
  padding: 18px;
  border-radius: 22px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.72);
  transition: transform var(--transition), box-shadow var(--transition), border-color var(--transition);
}

.recommend-card:hover {
  transform: translateY(-2px);
  border-color: rgba(37, 99, 235, 0.22);
  box-shadow: var(--shadow-sm);
}

.recommend-card strong {
  font-size: 18px;
  letter-spacing: -0.03em;
}

.recommend-card p,
.feedback-saved p {
  margin: 0;
  color: var(--ink-soft);
}

.recommend-link {
  color: var(--brand-deep);
  font-weight: 800;
}

.feedback-saved {
  display: grid;
  gap: 12px;
  padding: 18px;
  border-radius: 18px;
  background: rgba(15, 118, 110, 0.08);
  border: 1px solid rgba(15, 118, 110, 0.12);
}

.compact-state {
  padding: 24px 16px;
}

@media (max-width: 980px) {
  .study-hero,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
