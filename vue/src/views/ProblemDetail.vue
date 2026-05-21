<template>
  <div class="page">
    <section v-if="loading" class="loading-state">
      <strong>题目详情载入中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else-if="problem">
      <section class="page-hero detail-hero">
        <div class="stack">
          <span class="eyebrow">Problem #{{ problem.id }}</span>
          <div class="page-title">
            <div>
              <h1>{{ problem.title }}</h1>
              <p class="page-subtitle">详情页直接使用 `/api/problems/:id`，提交区对接 `/api/submit` 和 `/api/ws`。</p>
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
            <span v-if="!problem.tags.length" class="mini-tag">暂无标签</span>
          </div>
        </div>
      </section>

      <section class="detail-grid">
        <article class="card stack">
          <div class="section-title">
            <h2>题目描述</h2>
          </div>
          <div class="description-body" v-html="renderedDescription"></div>
        </article>

        <article class="card stack">
          <div class="section-title">
            <h2>提交代码</h2>
          </div>

          <div v-if="!store.isLoggedIn" class="empty-state compact">
            <strong>登录后才能提交</strong>
            <span class="muted">你的后端要求提交接口必须通过 JWT 鉴权。</span>
            <router-link to="/login" class="btn btn-primary">去登录</router-link>
          </div>

          <template v-else>
            <div class="field">
              <label for="language">语言</label>
              <select id="language" v-model="language" class="select">
                <option value="go">Go</option>
                <option value="python">Python</option>
                <option value="java">Java</option>
                <option value="cpp">C++</option>
              </select>
            </div>

            <div class="field">
              <label for="code">代码</label>
              <textarea
                id="code"
                v-model="code"
                class="textarea mono code-editor"
                :placeholder="placeholderByLanguage[language]"
              ></textarea>
            </div>

            <div class="cluster">
              <button class="btn btn-primary" :disabled="submitting || !code.trim()" @click="handleSubmit">
                <span v-if="submitting" class="spinner"></span>
                <span v-else>提交判题</span>
              </button>
              <button class="btn btn-outline" @click="code = placeholderByLanguage[language]">填入模板</button>
            </div>

            <div v-if="submitState" class="submit-feedback" :class="submitState.kind">
              <strong>{{ submitState.title }}</strong>
              <p>{{ submitState.message }}</p>
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
      <span class="muted">后端没有返回这道题，可能已经被管理员删除。</span>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getProblemDetail, getSubmission, submitCode } from '../api'
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

const placeholderByLanguage = {
  go: 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello GoJo")\n}',
  python: 'print("Hello GoJo")',
  java: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello GoJo");\n    }\n}',
  cpp: '#include <iostream>\nusing namespace std;\n\nint main() {\n    cout << "Hello GoJo" << endl;\n    return 0;\n}',
}

const renderedDescription = computed(() => {
  const text = problem.value?.description || ''
  const escaped = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  return escaped
    .replace(/```([\s\S]*?)```/g, '<pre class="code-block">$1</pre>')
    .replace(/\n/g, '<br>')
})

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
        message: submission.actualOutput || '可以点击下面的详情按钮查看完整结果。',
        submissionId: submission.id,
      }
      stopPolling()
      closeSocket()
      fetchProblem()
    }
  } catch {}
}

function openSocket(submissionId) {
  const token = store.token
  if (!token) {
    return
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  socket = new WebSocket(`${protocol}//${window.location.host}/api/ws?token=${encodeURIComponent(token)}`)

  socket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (Number(payload.submission_id) === submissionId) {
        refreshSubmission()
      }
    } catch {}
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

    openSocket(result.submissionId)
    stopPolling()
    pollTimer = setInterval(refreshSubmission, 2000)
  } catch (requestError) {
    submitState.value = {
      kind: 'danger',
      title: '提交失败',
      message: requestError.response?.data?.error || '后端没有接受这次提交。',
      submissionId: 0,
    }
  } finally {
    submitting.value = false
  }
}

onMounted(fetchProblem)
onUnmounted(() => {
  closeSocket()
  stopPolling()
})
</script>

<style scoped>
.detail-hero {
  padding: 28px;
}

.detail-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: 1.1fr 0.9fr;
}

.description-body {
  font-size: 15px;
  color: var(--ink);
}

.description-body :deep(pre) {
  margin: 18px 0;
  padding: 16px;
  overflow: auto;
  border-radius: 18px;
  background: #18243d;
  color: #f2f6ff;
}

.compact {
  padding: 24px 16px;
}

.code-editor {
  min-height: 320px;
  background: #18243d;
  color: #f2f6ff;
  border-color: rgba(255, 255, 255, 0.08);
}

.code-editor:focus {
  background: #1b2946;
}

.submit-feedback {
  display: grid;
  gap: 10px;
  padding: 18px;
  border-radius: 20px;
}

.submit-feedback strong {
  font-size: 18px;
}

.submit-feedback p {
  margin: 0;
  color: var(--ink-soft);
  white-space: pre-wrap;
  word-break: break-word;
}

.submit-feedback.pending {
  background: rgba(61, 115, 199, 0.12);
}

.submit-feedback.success {
  background: rgba(31, 143, 99, 0.12);
}

.submit-feedback.warning,
.submit-feedback.danger {
  background: rgba(187, 77, 58, 0.12);
}

@media (max-width: 900px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
