<template>
  <div class="page">
    <section class="page-hero hero-grid">
      <div class="hero-copy">
        <span class="eyebrow">Go Backend Demo · OJ Frontend</span>
        <div class="page-title">
          <div>
            <h1>让你的 Go 判题后端，有一套真正像作品集的前端门面。</h1>
            <p class="page-subtitle">
              题库、排行榜、提交链路、管理员后台都围绕你的后端接口重写，页面更精致，也更容易讲给面试官听。
            </p>
          </div>
        </div>
        <div class="cluster hero-actions">
          <router-link v-if="!store.isLoggedIn" to="/register" class="btn btn-primary">创建账号</router-link>
          <router-link v-if="!store.isLoggedIn" to="/login" class="btn btn-outline">已有账号，去登录</router-link>
          <router-link v-if="store.isLoggedIn" to="/profile" class="btn btn-primary">进入个人中心</router-link>
          <router-link to="/leaderboard" class="btn btn-secondary">查看排行榜</router-link>
        </div>
      </div>

      <div class="hero-side stack">
        <div class="metric-grid">
          <article class="metric-card">
            <span class="metric-value">{{ total }}</span>
            <span class="metric-label">后端可用题目总数</span>
          </article>
          <article class="metric-card">
            <span class="metric-value">{{ tags.length }}</span>
            <span class="metric-label">已挂载标签</span>
          </article>
          <article class="metric-card">
            <span class="metric-value">{{ visibleProblems.length }}</span>
            <span class="metric-label">当前页展示题目</span>
          </article>
        </div>

        <div v-if="store.isAdmin" class="admin-spotlight">
          <div>
            <strong>管理员快捷入口</strong>
            <p>你的后端已经区分了管理员路由，这里保留显眼入口，方便演示题目、标签和测试用例管理。</p>
          </div>
          <div class="cluster">
            <router-link to="/admin/problems" class="btn btn-outline btn-sm">题目管理</router-link>
            <router-link to="/admin/problems/new" class="btn btn-primary btn-sm">新建题目</router-link>
            <router-link to="/admin/tags" class="btn btn-secondary btn-sm">标签管理</router-link>
          </div>
        </div>
      </div>
    </section>

    <section class="glass-panel stack">
      <div class="toolbar">
        <div class="field search-field">
          <label for="problem-search">快速筛选</label>
          <input
            id="problem-search"
            v-model.trim="searchTerm"
            class="input"
            placeholder="按题目标题或 ID 过滤当前页"
          />
        </div>
        <div class="toolbar-summary">
          <strong>{{ total }}</strong>
          <span>道题目 · 第 {{ page }} / {{ totalPages }} 页</span>
        </div>
      </div>

      <div class="cluster">
        <button
          class="tag-pill"
          :class="{ active: !selectedTagId }"
          @click="applyTag(null)"
        >
          全部题目
        </button>
        <button
          v-for="tag in tags"
          :key="tag.id"
          class="tag-pill"
          :class="{ active: selectedTagId === tag.id }"
          @click="applyTag(tag.id)"
        >
          {{ tag.name }}
        </button>
      </div>
    </section>

    <section class="stack">
      <div class="section-title">
        <h2>题目列表</h2>
        <span class="muted">与 `/api/problems` 和 `/api/tags` 接口精确对齐</span>
      </div>

      <div v-if="loading" class="loading-state">
        <strong>题库加载中</strong>
        <span class="spinner spinner-dark"></span>
      </div>

      <div v-else-if="visibleProblems.length" class="problem-grid">
        <router-link
          v-for="problem in visibleProblems"
          :key="problem.id"
          :to="`/problems/${problem.id}`"
          class="problem-card"
        >
          <div class="problem-topline">
            <span class="problem-index">#{{ problem.id }}</span>
            <span v-if="problem.isAc" class="badge badge-success">已通过</span>
          </div>

          <h3>{{ problem.title }}</h3>

          <div class="cluster">
            <span v-for="tag in problem.tags" :key="tag.id" class="mini-tag">{{ tag.name }}</span>
            <span v-if="!problem.tags.length" class="mini-tag">未分类</span>
          </div>

          <div class="problem-meta">
            <span>提交 {{ problem.submitCount }}</span>
            <span>通过 {{ problem.acceptedCount }}</span>
            <strong>{{ getAcceptanceRate(problem) }}%</strong>
          </div>
        </router-link>
      </div>

      <div v-else class="empty-state">
        <strong>这一页暂时没有符合条件的题目</strong>
        <span class="muted">可以切换标签，或者清空筛选条件重新看看。</span>
      </div>

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
  return problems.value.filter((problem) => {
    return problem.title.toLowerCase().includes(keyword) || String(problem.id).includes(keyword)
  })
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
  gap: 24px;
  padding: 34px;
  grid-template-columns: 1.35fr 1fr;
}

.hero-copy {
  display: grid;
  align-content: start;
  gap: 18px;
}

.hero-actions {
  margin-top: 8px;
}

.admin-spotlight {
  display: grid;
  gap: 14px;
  padding: 22px;
  border-radius: 28px;
  background: linear-gradient(135deg, rgba(203, 109, 67, 0.12), rgba(255, 255, 255, 0.6));
  border: 1px solid rgba(203, 109, 67, 0.16);
}

.admin-spotlight strong {
  font-size: 18px;
}

.admin-spotlight p {
  margin: 8px 0 0;
  color: var(--ink-soft);
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 16px;
  align-items: end;
}

.search-field {
  flex: 1;
  min-width: 240px;
}

.toolbar-summary {
  display: grid;
  justify-items: end;
  gap: 4px;
  min-width: 180px;
}

.toolbar-summary strong {
  font-size: 28px;
  letter-spacing: -0.05em;
}

.tag-pill {
  padding: 10px 15px;
  border-radius: 999px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.66);
  color: var(--ink-soft);
  cursor: pointer;
  font-weight: 700;
  transition: all var(--transition);
}

.tag-pill.active,
.tag-pill:hover {
  background: linear-gradient(135deg, var(--accent), #115c56);
  border-color: transparent;
  color: #f4fffd;
}

.problem-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
}

.problem-card {
  display: grid;
  gap: 16px;
  padding: 22px;
  border: 1px solid var(--line);
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.68);
  box-shadow: var(--shadow-sm);
  transition: transform var(--transition), box-shadow var(--transition), border-color var(--transition);
}

.problem-card:hover {
  transform: translateY(-4px);
  border-color: rgba(203, 109, 67, 0.24);
  box-shadow: var(--shadow-md);
}

.problem-card h3 {
  margin: 0;
  font-size: 20px;
  line-height: 1.25;
  letter-spacing: -0.03em;
}

.problem-topline,
.problem-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.problem-index {
  font-size: 13px;
  font-weight: 800;
  color: var(--brand-deep);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.problem-meta {
  color: var(--ink-soft);
  font-size: 14px;
}

.problem-meta strong {
  color: var(--accent);
}

@media (max-width: 900px) {
  .hero-grid {
    grid-template-columns: 1fr;
    padding: 24px;
  }

  .toolbar-summary {
    justify-items: start;
  }
}
</style>
