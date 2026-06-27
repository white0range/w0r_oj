<template>
  <div class="page">
    <section class="page-hero hero-grid">
      <div class="hero-copy">
        <span class="eyebrow">Online Judge Workspace</span>
        <div class="page-title">
          <div>
            <h1>面向刷题、训练与复盘的一体化 OJ 控制台</h1>
            <p class="page-subtitle">
              这不是单纯的题目列表页。你可以在这里筛题、查看通过率、进入提交与结果页，并把它和后端的异步判题、排行榜、AI 分析链路串起来。
            </p>
          </div>
        </div>
        <div class="cluster hero-actions">
          <router-link to="/" class="btn btn-primary">开始刷题</router-link>
          <router-link to="/leaderboard" class="btn btn-outline">查看排行榜</router-link>
          <router-link v-if="store.isLoggedIn" to="/my-submissions" class="btn btn-ghost">我的提交</router-link>
          <router-link v-else to="/register" class="btn btn-secondary">创建账号</router-link>
        </div>
      </div>

      <div class="hero-side stack">
        <section class="hero-insight">
          <div class="insight-head">
            <strong>Today on Gojo OJ</strong>
            <span class="mini-tag">Live Dataset</span>
          </div>
          <div class="metric-grid">
            <article class="metric-card">
              <span class="metric-value">{{ total }}</span>
              <span class="metric-label">题目总数</span>
            </article>
            <article class="metric-card">
              <span class="metric-value">{{ tags.length }}</span>
              <span class="metric-label">标签数量</span>
            </article>
            <article class="metric-card">
              <span class="metric-value">{{ visibleProblems.length }}</span>
              <span class="metric-label">当前筛选结果</span>
            </article>
          </div>
        </section>

        <section class="hero-note">
          <strong>Professional OJ Layout</strong>
          <p>
            首页聚焦题库运营视角：筛选面板、题目表、通过率、状态标记和分页都围绕高频做题场景组织，而不是纯展示型 landing page。
          </p>
        </section>
      </div>
    </section>

    <section class="glass-panel control-panel">
      <div class="filter-main">
        <div class="field search-field">
          <label for="problem-search">搜索题目</label>
          <input
            id="problem-search"
            v-model.trim="searchTerm"
            class="input"
            placeholder="按题号、标题或关键字快速过滤当前页"
          />
        </div>
        <div class="summary-box">
          <strong>{{ total }}</strong>
          <span>题目 · 第 {{ page }} / {{ totalPages }} 页</span>
        </div>
      </div>
      <div class="cluster filter-tags">
        <button class="tag-toggle" :class="{ active: !selectedTagId }" @click="applyTag(null)">全部题目</button>
        <button
          v-for="tag in tags"
          :key="tag.id"
          class="tag-toggle"
          :class="{ active: selectedTagId === tag.id }"
          @click="applyTag(tag.id)"
        >
          {{ tag.name }}
        </button>
      </div>
    </section>

    <section class="stack">
      <div class="section-title">
        <h2>题库总览</h2>
        <span class="muted">点击任意题目进入详情页并提交代码</span>
      </div>

      <div v-if="loading" class="loading-state">
        <strong>题库加载中</strong>
        <span class="spinner spinner-dark"></span>
      </div>

      <template v-else-if="visibleProblems.length">
        <section class="problem-table card">
          <div class="problem-row problem-head">
            <span>题号</span>
            <span>题目</span>
            <span>标签</span>
            <span>限制</span>
            <span>通过率</span>
          </div>
          <router-link
            v-for="problem in visibleProblems"
            :key="problem.id"
            :to="`/problems/${problem.id}`"
            class="problem-row problem-item"
          >
            <div class="problem-id">
              <strong>#{{ problem.id }}</strong>
              <span v-if="problem.isAc" class="badge badge-success">已通过</span>
            </div>
            <div class="problem-main">
              <strong>{{ problem.title }}</strong>
              <span>{{ problem.submitCount }} 次提交 · {{ problem.acceptedCount }} 次通过</span>
            </div>
            <div class="problem-tags">
              <span v-for="tag in problem.tags" :key="tag.id" class="mini-tag">{{ tag.name }}</span>
              <span v-if="!problem.tags.length" class="mini-tag">未分类</span>
            </div>
            <div class="problem-limit">
              <span>{{ problem.timeLimit }} ms</span>
              <span>{{ problem.memoryLimit }} MB</span>
            </div>
            <div class="problem-rate">
              <strong>{{ getAcceptanceRate(problem) }}%</strong>
              <small>acceptance</small>
            </div>
          </router-link>
        </section>

        <div v-if="totalPages > 1" class="pagination">
          <button class="page-chip" :disabled="page <= 1" @click="changePage(page - 1)">上一页</button>
          <button
            v-for="pageNumber in pagesToShow"
            :key="pageNumber"
            class="page-chip"
            :class="{ active: pageNumber === page }"
            @click="changePage(pageNumber)"
          >
            {{ pageNumber }}
          </button>
          <button class="page-chip" :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</button>
        </div>
      </template>

      <div v-else class="empty-state">
        <strong>没有符合条件的题目</strong>
        <span class="muted">可以切换标签或清空搜索关键字后再试。</span>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getProblems, getTags } from '../api'
import { store } from '../store'
import { getAcceptanceRate } from '../utils/normalizers'

const problems = ref([])
const tags = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(12)
const selectedTagId = ref(null)
const searchTerm = ref('')
const loading = ref(true)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))

const pagesToShow = computed(() => {
  const items = []
  const start = Math.max(1, page.value - 2)
  const end = Math.min(totalPages.value, page.value + 2)

  for (let index = start; index <= end; index += 1) {
    items.push(index)
  }

  return items
})

const visibleProblems = computed(() => {
  if (!searchTerm.value) {
    return problems.value
  }

  const keyword = searchTerm.value.toLowerCase()
  return problems.value.filter((problem) => problem.title.toLowerCase().includes(keyword) || String(problem.id).includes(keyword))
})

async function fetchProblems() {
  loading.value = true

  try {
    const data = await getProblems({
      page: page.value,
      limit: limit.value,
      ...(selectedTagId.value ? { tag_id: selectedTagId.value } : {}),
    })

    problems.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function fetchTags() {
  tags.value = await getTags()
}

function applyTag(tagId) {
  selectedTagId.value = tagId
  page.value = 1
  fetchProblems()
}

function changePage(nextPage) {
  if (nextPage < 1 || nextPage > totalPages.value) {
    return
  }

  page.value = nextPage
  fetchProblems()
}

onMounted(async () => {
  await Promise.all([fetchProblems(), fetchTags()])
})
</script>

<style scoped>
.hero-grid {
  display: grid;
  grid-template-columns: 1.3fr 0.9fr;
  gap: 22px;
}

.hero-copy {
  display: grid;
  gap: 18px;
  align-content: start;
}

.hero-actions {
  margin-top: 4px;
}

.hero-side {
  align-content: start;
}

.hero-insight,
.hero-note {
  padding: 22px;
  border: 1px solid var(--line);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.72);
}

.insight-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.hero-note p {
  margin: 8px 0 0;
  color: var(--ink-soft);
}

.control-panel {
  display: grid;
  gap: 16px;
}

.filter-main {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.search-field {
  flex: 1;
  min-width: 260px;
}

.summary-box {
  display: grid;
  gap: 4px;
  min-width: 180px;
  justify-items: end;
}

.summary-box strong {
  font-size: 28px;
  letter-spacing: -0.05em;
}

.problem-table {
  padding: 0;
  overflow: hidden;
}

.problem-row {
  display: grid;
  grid-template-columns: 120px 1.4fr 1.2fr 160px 120px;
  gap: 16px;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
}

.problem-row:last-child {
  border-bottom: 0;
}

.problem-head {
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--ink-faint);
}

.problem-item {
  transition: background var(--transition);
}

.problem-item:hover {
  background: rgba(37, 99, 235, 0.04);
}

.problem-id,
.problem-main,
.problem-limit,
.problem-rate {
  display: grid;
  gap: 6px;
}

.problem-main strong,
.problem-rate strong {
  font-size: 16px;
  letter-spacing: -0.02em;
}

.problem-main span,
.problem-limit span,
.problem-rate small {
  color: var(--ink-soft);
}

.problem-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 1040px) {
  .hero-grid {
    grid-template-columns: 1fr;
  }

  .summary-box {
    justify-items: start;
  }

  .problem-row {
    grid-template-columns: 90px 1fr;
  }

  .problem-head {
    display: none;
  }

  .problem-row > :nth-child(3),
  .problem-row > :nth-child(4),
  .problem-row > :nth-child(5) {
    grid-column: 2 / -1;
  }
}
</style>
