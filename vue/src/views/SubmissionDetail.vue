<template>
  <div class="page">
    <section v-if="loading" class="loading-state">
      <strong>提交详情载入中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else-if="submission">
      <section class="page-hero submission-hero">
        <div>
          <span class="eyebrow">Submission #{{ submission.id }}</span>
          <div class="page-title">
            <div>
              <h1>{{ submission.status }}</h1>
              <p class="page-subtitle">详情页直接使用 `/api/submissions/:id`。如果状态还是 `Pending`，页面会自动轮询刷新。</p>
            </div>
          </div>
          <div class="cluster">
            <span class="pill">题目 #{{ submission.problemId }}</span>
            <span class="pill">{{ submission.language.toUpperCase() }}</span>
            <span class="pill">提交 ID {{ submission.id }}</span>
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

const route = useRoute()
const routeId = computed(() => route.params.id)
const loading = ref(true)
const submission = ref(null)
let pollTimer = null

async function fetchSubmission() {
  try {
    submission.value = await getSubmission(routeId.value)
  } catch {
    submission.value = null
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await fetchSubmission()

  if (submission.value?.status === 'Pending') {
    pollTimer = setInterval(async () => {
      await fetchSubmission()
      if (submission.value?.status !== 'Pending') {
        clearInterval(pollTimer)
        pollTimer = null
      }
    }, 2000)
  }
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
  }
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
  border-radius: 20px;
  background: var(--surface-dark);
  color: #eef4ff;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.output-view {
  background: #0e1a30;
  color: #d5e3ff;
}

@media (max-width: 900px) {
  .submission-hero,
  .detail-grid {
    grid-template-columns: 1fr;
  }

  .submission-hero {
    flex-direction: column;
  }
}
</style>
