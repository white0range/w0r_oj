<template>
  <div class="page workspace-page">
    <section class="workspace-masthead">
      <div class="workspace-copy">
        <span class="workspace-kicker">Problem Workspace</span>
        <h1>围绕题库、筛选与提交流转的 OJ 工作台</h1>
        <p>
          从这里进入日常刷题流程：按标签筛选题目、快速查看通过率与限制、定位当前页结果，
          再进入题目详情完成代码提交与复盘。
        </p>
        <div class="cluster workspace-actions">
          <a href="#problem-explorer" class="btn btn-primary">浏览题库</a>
          <router-link to="/leaderboard" class="btn btn-outline">查看排行榜</router-link>
          <router-link v-if="store.isLoggedIn" to="/study-plan" class="btn btn-ghost">打开 AI 学习助手</router-link>
          <router-link v-else to="/register" class="btn btn-secondary">创建账户</router-link>
        </div>
      </div>

      <aside class="workspace-overview">
        <article class="overview-card overview-card-strong">
          <span class="overview-label">题库总量</span>
          <strong>{{ total }}</strong>
          <span class="overview-meta">持续维护中的在线题单</span>
        </article>
        <article class="overview-card">
          <span class="overview-label">标签数量</span>
          <strong>{{ tags.length }}</strong>
          <span class="overview-meta">按知识点组织筛选入口</span>
        </article>
        <article class="overview-card">
          <span class="overview-label">当前页结果</span>
          <strong>{{ visibleProblems.length }}</strong>
          <span class="overview-meta">结合筛选和关键词的即时结果</span>
        </article>
        <article class="overview-card">
          <span class="overview-label">当前页已通过</span>
          <strong>{{ solvedVisibleCount }}</strong>
          <span class="overview-meta">只统计当前列表中已 AC 的题目</span>
        </article>
      </aside>
    </section>

    <section id="problem-explorer" class="workspace-grid">
      <section class="workspace-primary card">
        <div class="explorer-head">
          <div>
            <span class="section-kicker">Problem Explorer</span>
            <h2>题库浏览</h2>
            <p>关键词过滤只作用于当前页，标签筛选会请求后端刷新题目列表。</p>
          </div>
          <div class="explorer-summary">
            <strong>{{ total }}</strong>
            <span>题目总数</span>
            <small>第 {{ page }} / {{ totalPages }} 页</small>
          </div>
        </div>

        <div class="explorer-toolbar">
          <div class="field search-field">
            <label for="problem-search">搜索题目</label>
            <input
              id="problem-search"
              v-model.trim="searchTerm"
              class="input"
              placeholder="按题号、标题或关键字过滤当前页"
            />
          </div>

          <div class="toolbar-stats">
            <div>
              <span class="toolbar-stat-label">当前标签</span>
              <strong>{{ activeTagLabel }}</strong>
            </div>
            <div>
              <span class="toolbar-stat-label">当前页命中</span>
              <strong>{{ visibleProblems.length }}</strong>
            </div>
          </div>
        </div>

        <div class="filter-strip">
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

        <div v-if="loading" class="loading-state">
          <strong>题库加载中</strong>
          <span class="spinner spinner-dark"></span>
        </div>

        <template v-else-if="visibleProblems.length">
          <section class="problem-table workspace-table">
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

          <div v-if="totalPages > 1" class="pagination workspace-pagination">
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

      <aside class="workspace-secondary stack">
        <section class="workspace-panel">
          <div class="panel-head">
            <span class="section-kicker">Current Filter</span>
            <h3>当前筛选状态</h3>
          </div>
          <dl class="filter-summary">
            <div>
              <dt>标签</dt>
              <dd>{{ activeTagLabel }}</dd>
            </div>
            <div>
              <dt>关键词</dt>
              <dd>{{ searchTerm || '未输入' }}</dd>
            </div>
            <div>
              <dt>分页</dt>
              <dd>第 {{ page }} / {{ totalPages }} 页</dd>
            </div>
            <div>
              <dt>本页通过</dt>
              <dd>{{ solvedVisibleCount }} / {{ visibleProblems.length || 0 }}</dd>
            </div>
          </dl>
        </section>

        <section class="workspace-panel">
          <div class="panel-head">
            <span class="section-kicker">Popular Tags</span>
            <h3>常用标签</h3>
          </div>
          <div class="cluster panel-tags">
            <button
              v-for="tag in topTags"
              :key="`top-${tag.id}`"
              class="tag-toggle panel-tag"
              :class="{ active: selectedTagId === tag.id }"
              @click="applyTag(tag.id)"
            >
              {{ tag.name }}
            </button>
          </div>
        </section>

        <section class="workspace-panel" v-if="featuredProblems.length">
          <div class="panel-head">
            <span class="section-kicker">Starter Picks</span>
            <h3>当前页优先查看</h3>
          </div>
          <div class="featured-list">
            <router-link
              v-for="problem in featuredProblems"
              :key="`featured-${problem.id}`"
              :to="`/problems/${problem.id}`"
              class="featured-problem"
            >
              <div>
                <strong>#{{ problem.id }} {{ problem.title }}</strong>
                <span>{{ getAcceptanceRate(problem) }}% 通过率 · {{ problem.timeLimit }} ms</span>
              </div>
              <span class="featured-link">查看题目</span>
            </router-link>
          </div>
        </section>
      </aside>
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

const activeTagLabel = computed(() => {
  if (!selectedTagId.value) {
    return '全部题目'
  }
  return tags.value.find((tag) => tag.id === selectedTagId.value)?.name || '未知标签'
})

const solvedVisibleCount = computed(() => visibleProblems.value.filter((problem) => problem.isAc).length)

const topTags = computed(() => tags.value.slice(0, 12))

const featuredProblems = computed(() => visibleProblems.value.slice(0, 3))

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
.workspace-page {
  gap: 24px;
}

.workspace-masthead {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(320px, 0.8fr);
  gap: 20px;
  padding: 28px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  border-radius: 30px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(244, 248, 255, 0.9)),
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.12), transparent 30%);
  box-shadow: var(--shadow-md);
}

.workspace-copy {
  display: grid;
  align-content: start;
  gap: 18px;
}

.workspace-kicker,
.section-kicker {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  padding: 7px 12px;
  border-radius: 999px;
  background: rgba(15, 23, 40, 0.06);
  color: var(--ink-soft);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.workspace-copy h1 {
  margin: 0;
  max-width: 780px;
  font-size: clamp(38px, 4.8vw, 64px);
  line-height: 0.98;
  letter-spacing: -0.06em;
}

.workspace-copy p {
  margin: 0;
  max-width: 760px;
  color: var(--ink-soft);
  font-size: 16px;
}

.workspace-actions {
  margin-top: 6px;
}

.workspace-overview {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.overview-card {
  display: grid;
  gap: 10px;
  min-height: 152px;
  padding: 20px;
  border: 1px solid rgba(15, 23, 40, 0.08);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.82);
}

.overview-card-strong {
  background: linear-gradient(145deg, rgba(29, 78, 216, 0.98), rgba(37, 99, 235, 0.88));
  color: #f8fbff;
}

.overview-card-strong .overview-label,
.overview-card-strong .overview-meta {
  color: rgba(248, 251, 255, 0.8);
}

.overview-label {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--ink-faint);
}

.overview-card strong {
  font-size: clamp(30px, 4vw, 42px);
  letter-spacing: -0.05em;
}

.overview-meta {
  color: var(--ink-soft);
  font-size: 13px;
}

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) 340px;
  gap: 20px;
  align-items: start;
}

.workspace-primary {
  display: grid;
  gap: 18px;
  padding: 22px;
}

.explorer-head {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
  flex-wrap: wrap;
}

.explorer-head h2,
.panel-head h3 {
  margin: 8px 0 0;
  font-size: 26px;
  letter-spacing: -0.04em;
}

.explorer-head p,
.panel-head p {
  margin: 8px 0 0;
  color: var(--ink-soft);
}

.explorer-summary {
  display: grid;
  gap: 2px;
  min-width: 180px;
  justify-items: end;
}

.explorer-summary strong {
  font-size: 30px;
  letter-spacing: -0.05em;
}

.explorer-summary span,
.explorer-summary small,
.toolbar-stat-label {
  color: var(--ink-soft);
}

.explorer-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  align-items: end;
}

.search-field {
  min-width: 280px;
}

.toolbar-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(120px, 1fr));
  gap: 12px;
}

.toolbar-stats div {
  display: grid;
  gap: 4px;
  padding: 14px 16px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: rgba(245, 248, 255, 0.9);
}

.toolbar-stats strong {
  font-size: 18px;
  letter-spacing: -0.03em;
}

.filter-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding-bottom: 4px;
}

.workspace-table {
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.84);
}

.problem-row {
  display: grid;
  grid-template-columns: 110px 1.5fr 1.1fr 150px 110px;
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
  background: rgba(245, 248, 255, 0.95);
}

.problem-item {
  transition: background var(--transition), transform var(--transition);
}

.problem-item:hover {
  background: rgba(37, 99, 235, 0.04);
  transform: translateY(-1px);
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

.workspace-pagination {
  justify-content: flex-end;
}

.workspace-secondary {
  gap: 16px;
  position: sticky;
  top: 88px;
}

.workspace-panel {
  display: grid;
  gap: 14px;
  padding: 20px;
  border: 1px solid var(--line);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: var(--shadow-sm);
}

.panel-head {
  display: grid;
  gap: 2px;
}

.filter-summary {
  display: grid;
  gap: 12px;
  margin: 0;
}

.filter-summary div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(15, 23, 40, 0.08);
}

.filter-summary div:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}

.filter-summary dt {
  color: var(--ink-faint);
}

.filter-summary dd {
  margin: 0;
  text-align: right;
  font-weight: 700;
}

.panel-tags {
  gap: 8px;
}

.panel-tag {
  padding-inline: 12px;
}

.featured-list {
  display: grid;
  gap: 12px;
}

.featured-problem {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: rgba(245, 248, 255, 0.92);
  transition: transform var(--transition), border-color var(--transition), box-shadow var(--transition);
}

.featured-problem:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.24);
  box-shadow: var(--shadow-sm);
}

.featured-problem div {
  display: grid;
  gap: 4px;
}

.featured-problem strong {
  font-size: 15px;
  letter-spacing: -0.02em;
}

.featured-problem span {
  color: var(--ink-soft);
  font-size: 13px;
}

.featured-link {
  font-weight: 800;
  color: var(--brand-deep);
  white-space: nowrap;
}

@media (max-width: 1180px) {
  .workspace-masthead,
  .workspace-grid,
  .explorer-toolbar {
    grid-template-columns: 1fr;
  }

  .workspace-secondary {
    position: static;
  }

  .explorer-summary {
    justify-items: start;
  }
}

@media (max-width: 860px) {
  .workspace-overview {
    grid-template-columns: 1fr 1fr;
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

@media (max-width: 640px) {
  .workspace-masthead {
    padding: 20px;
  }

  .workspace-copy h1 {
    font-size: 34px;
  }

  .workspace-overview,
  .toolbar-stats {
    grid-template-columns: 1fr;
  }
}
</style>
