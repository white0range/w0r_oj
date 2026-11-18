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
              <p class="page-subtitle">回顾这次解法，查看运行结果与资源消耗。</p>
            </div>
          </div>
          <div class="cluster">
            <span class="pill">题目 #{{ submission.problemId }}</span>
            <span class="pill">{{ submission.language.toUpperCase() }}</span>
            <span class="pill">耗时 {{ submission.timeCost }} ms</span>
            <span class="pill">内存 {{ submission.memoryCost }} KB</span>
          </div>
        </div>
        <router-link to="/my-submissions" class="btn btn-outline">返回提交列表</router-link>
      </section>

      <section class="detail-grid">
        <article class="card stack">
          <div class="section-title">
            <h2>提交代码</h2>
          </div>
          <pre class="code-view mono">{{ submission.code || '暂无代码内容。' }}</pre>
        </article>

        <article class="card stack">
          <div class="section-title">
            <h2>判题说明</h2>
          </div>
          <pre class="output-view mono">{{ submission.actualOutput || '暂无判题说明。' }}</pre>
        </article>
      </section>
    </template>

    <section v-else-if="loadFailed" class="empty-state" role="status">
      <strong>暂时无法获取判题状态</strong>
      <span class="muted">正在自动重试。</span>
    </section>

    <section v-else class="empty-state">
      <strong>这条提交不存在</strong>
      <span class="muted">可能已经失效，或者你没有权限查看它。</span>
    </section>
  </div>
</template>

<script setup>
import { onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { watchSubmissionStatus } from '../utils/submissionStatus'

const route = useRoute()
const loading = ref(true)
const loadFailed = ref(false)
const submission = ref(null)
let stopWatchingSubmission

watch(() => route.name === 'submission-detail' ? route.params.id : null, (id) => {
  stopWatchingSubmission?.()
  if (!id) return
  loading.value = true
  loadFailed.value = false
  submission.value = null
  stopWatchingSubmission = watchSubmissionStatus(id, (result) => {
    submission.value = result
    loading.value = false
    loadFailed.value = false
  }, (_, retrying) => {
    loading.value = false
    loadFailed.value = retrying
    if (!retrying) submission.value = null
  })
}, { immediate: true })

onUnmounted(() => stopWatchingSubmission?.())
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
  border-radius: var(--radius-sm);
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
