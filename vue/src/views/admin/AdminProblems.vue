<template>
  <div class="page">
    <section class="page-hero admin-hero">
      <div>
        <span class="eyebrow">Problem Center</span>
        <div class="page-title">
          <div>
            <h1>题目管理</h1>
            <p class="page-subtitle">
              在这里统一管理题目、标签与题面质量。列表沿用真实数据接口，适合作为完整后台能力展示。
            </p>
          </div>
        </div>
      </div>
      <div class="cluster">
        <router-link to="/admin/tags" class="btn btn-outline">标签管理</router-link>
        <router-link to="/admin/problems/new" class="btn btn-primary">新建题目</router-link>
      </div>
    </section>

    <section class="metric-grid">
      <article class="metric-card">
        <span class="metric-value">{{ total }}</span>
        <span class="metric-label">题目总数</span>
      </article>
      <article class="metric-card">
        <span class="metric-value">{{ problems.length }}</span>
        <span class="metric-label">当前页题目</span>
      </article>
      <article class="metric-card">
        <span class="metric-value">{{ totalPages }}</span>
        <span class="metric-label">分页页数</span>
      </article>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>题目列表加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else>
      <section v-if="problems.length" class="card admin-table">
        <div class="admin-row admin-head">
          <span>ID</span>
          <span>题目</span>
          <span>标签</span>
          <span>提交 / 通过</span>
          <span>操作</span>
        </div>
        <div v-for="problem in problems" :key="problem.id" class="admin-row">
          <span>#{{ problem.id }}</span>
          <div class="stack compact-gap">
            <strong>{{ problem.title }}</strong>
            <span class="muted">通过率 {{ getAcceptanceRate(problem) }}%</span>
          </div>
          <div class="cluster">
            <span v-for="tag in problem.tags" :key="tag.id" class="mini-tag">{{ tag.name }}</span>
            <span v-if="!problem.tags.length" class="mini-tag">未分类</span>
          </div>
          <span>{{ problem.submitCount }} / {{ problem.acceptedCount }}</span>
          <div class="cluster">
            <router-link :to="`/admin/problems/${problem.id}/edit`" class="btn btn-ghost btn-sm">编辑</router-link>
            <button class="btn btn-danger btn-sm" @click="openDelete(problem)">删除</button>
          </div>
        </div>
      </section>

      <section v-else class="empty-state">
        <strong>还没有题目</strong>
        <span class="muted">先创建第一道题，再回到这里查看完整的后台管理效果。</span>
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

    <transition name="fade-slide">
      <div v-if="deletingTarget" class="dialog-backdrop" @click.self="deletingTarget = null">
        <div class="dialog-card">
          <h3>确认删除题目？</h3>
          <p>
            题目《{{ deletingTarget.title }}》将从管理列表中移除。这里保持和后端当前行为一致，不额外夸大删除影响。
          </p>
          <div class="cluster">
            <button class="btn btn-ghost" @click="deletingTarget = null">取消</button>
            <button class="btn btn-danger" :disabled="deleting" @click="confirmDelete">
              <span v-if="deleting" class="spinner"></span>
              <span v-else>确认删除</span>
            </button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { adminDeleteProblem, getProblems } from '../../api'
import { getAcceptanceRate } from '../../utils/normalizers'

const loading = ref(true)
const deleting = ref(false)
const problems = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(12)
const deletingTarget = ref(null)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))

const pagesToShow = computed(() => {
  const list = []
  const start = Math.max(1, page.value - 2)
  const end = Math.min(totalPages.value, page.value + 2)

  for (let index = start; index <= end; index += 1) {
    list.push(index)
  }

  return list
})

async function fetchProblems() {
  loading.value = true

  try {
    const data = await getProblems({ page: page.value, limit: limit.value })
    problems.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function changePage(nextPage) {
  if (nextPage < 1 || nextPage > totalPages.value) {
    return
  }

  page.value = nextPage
  fetchProblems()
}

function openDelete(problem) {
  deletingTarget.value = problem
}

async function confirmDelete() {
  if (!deletingTarget.value) {
    return
  }

  deleting.value = true

  try {
    await adminDeleteProblem(deletingTarget.value.id)
    deletingTarget.value = null
    await fetchProblems()
  } finally {
    deleting.value = false
  }
}

onMounted(fetchProblems)
</script>

<style scoped>
.admin-hero {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 18px;
}

.admin-table {
  padding: 0;
  overflow: hidden;
}

.admin-row {
  display: grid;
  grid-template-columns: 90px 1.2fr 1fr 150px 210px;
  gap: 16px;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
}

.admin-row:last-child {
  border-bottom: 0;
}

.admin-head {
  font-size: 13px;
  font-weight: 700;
  color: var(--ink-faint);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.compact-gap {
  gap: 6px;
}

.dialog-backdrop {
  position: fixed;
  inset: 0;
  z-index: 40;
  display: grid;
  place-items: center;
  background: rgba(19, 35, 63, 0.28);
  backdrop-filter: blur(10px);
}

.dialog-card {
  width: min(92vw, 460px);
  padding: 24px;
  border-radius: 28px;
  background: #fffdfa;
  box-shadow: var(--shadow-lg);
}

.dialog-card h3 {
  margin-top: 0;
  margin-bottom: 12px;
}

.dialog-card p {
  margin: 0 0 18px;
  color: var(--ink-soft);
}

@media (max-width: 980px) {
  .admin-row {
    grid-template-columns: 70px 1fr;
  }

  .admin-head {
    display: none;
  }

  .admin-row > :nth-child(3),
  .admin-row > :nth-child(4),
  .admin-row > :nth-child(5) {
    grid-column: 2 / -1;
  }

  .admin-hero {
    flex-direction: column;
    align-items: start;
  }
}
</style>
