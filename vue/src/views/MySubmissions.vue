<template>
  <div class="page">
    <section class="page-hero submissions-hero">
      <div>
        <span class="eyebrow">Submissions</span>
        <div class="page-title">
          <div>
            <h1>我的提交</h1>
            <p class="page-subtitle">按状态筛选个人提交记录，查看每次判题的结果入口与基础元数据。</p>
          </div>
        </div>
      </div>
      <div class="hero-summary">
        <strong>{{ total }}</strong>
        <span>总提交数</span>
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>提交记录加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else>
      <section v-if="filteredItems.length" class="stack">
        <div class="glass-panel submissions-toolbar">
          <div class="cluster">
            <button
              v-for="option in filters"
              :key="option.value"
              class="tag-toggle"
              :class="{ active: statusFilter === option.value }"
              @click="statusFilter = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <span class="muted">第 {{ page }} / {{ totalPages }} 页</span>
        </div>

        <section class="card submission-table">
          <div class="submission-row submission-head">
            <span>提交</span>
            <span>题目</span>
            <span>语言</span>
            <span>状态</span>
            <span>时间</span>
          </div>
          <router-link
            v-for="item in filteredItems"
            :key="item.id"
            :to="`/submissions/${item.id}`"
            class="submission-row submission-item"
          >
            <div class="submission-cell">
              <strong>#{{ item.id }}</strong>
              <small>{{ item.problemId }}</small>
            </div>
            <div class="submission-main">
              <strong>{{ item.problemTitle || `题目 #${item.problemId}` }}</strong>
              <span>{{ item.actualOutput ? '含结果输出' : '等待查看详情' }}</span>
            </div>
            <span class="pill">{{ item.language.toUpperCase() }}</span>
            <span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span>
            <span class="submission-time">{{ formatTime(item.createdAt) }}</span>
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
      </section>

      <section v-else class="empty-state">
        <strong>还没有提交记录</strong>
        <span class="muted">从题库选择一道题，提交第一份代码后这里就会开始累积数据。</span>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getMySubmissions } from '../api'

const items = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(12)
const loading = ref(true)
const statusFilter = ref('ALL')

const filters = [
  { label: '全部', value: 'ALL' },
  { label: 'AC', value: 'AC' },
  { label: 'Pending', value: 'Pending' },
  { label: 'WA', value: 'WA' },
  { label: 'RE', value: 'RE' },
]

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))

const pagesToShow = computed(() => {
  const itemsToShow = []
  const start = Math.max(1, page.value - 2)
  const end = Math.min(totalPages.value, page.value + 2)

  for (let index = start; index <= end; index += 1) {
    itemsToShow.push(index)
  }

  return itemsToShow
})

const filteredItems = computed(() => {
  if (statusFilter.value === 'ALL') {
    return items.value
  }
  return items.value.filter((item) => item.status === statusFilter.value)
})

function statusClass(status) {
  return `status-${status || 'Pending'}`
}

function formatTime(value) {
  if (!value) {
    return '时间未知'
  }
  return new Date(value).toLocaleString('zh-CN')
}

async function fetchItems() {
  loading.value = true

  try {
    const data = await getMySubmissions({ page: page.value, limit: limit.value })
    items.value = data.items
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
  fetchItems()
}

onMounted(fetchItems)
</script>

<style scoped>
.submissions-hero {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
}

.hero-summary {
  display: grid;
  justify-items: end;
}

.hero-summary strong {
  font-size: 34px;
  letter-spacing: -0.05em;
}

.submissions-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.submission-table {
  padding: 0;
  overflow: hidden;
}

.submission-row {
  display: grid;
  grid-template-columns: 100px 1fr 120px 120px 190px;
  gap: 16px;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
}

.submission-row:last-child {
  border-bottom: 0;
}

.submission-head {
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--ink-faint);
}

.submission-item:hover {
  background: rgba(37, 99, 235, 0.04);
}

.submission-cell,
.submission-main {
  display: grid;
  gap: 6px;
}

.submission-main span,
.submission-time,
.submission-cell small {
  color: var(--ink-soft);
}

.status-pill {
  width: fit-content;
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(37, 99, 235, 0.12);
  font-weight: 800;
}

@media (max-width: 980px) {
  .submissions-hero {
    flex-direction: column;
    align-items: start;
  }

  .hero-summary {
    justify-items: start;
  }

  .submissions-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .submission-row {
    grid-template-columns: 90px 1fr;
  }

  .submission-head {
    display: none;
  }

  .submission-row > :nth-child(3),
  .submission-row > :nth-child(4),
  .submission-row > :nth-child(5) {
    grid-column: 2 / -1;
  }
}
</style>
