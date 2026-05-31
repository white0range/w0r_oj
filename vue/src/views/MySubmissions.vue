<template>
  <div class="page">
    <section class="page-hero page-title-block">
      <span class="eyebrow">Submissions</span>
      <div class="page-title">
        <div>
          <h1>我的提交</h1>
          <p class="page-subtitle">这里对接 `/api/my-submissions`，展示当前登录用户的提交历史、状态和查看入口。</p>
        </div>
        <div class="hero-mini">
          <strong>{{ total }}</strong>
          <span>条记录</span>
        </div>
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>提交记录载入中</strong>
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

        <div class="submission-list">
          <router-link
            v-for="item in filteredItems"
            :key="item.id"
            :to="`/submissions/${item.id}`"
            class="submission-card"
          >
            <div class="submission-main">
              <div>
                <span class="submission-id">#{{ item.id }}</span>
                <h3>{{ item.problemTitle || `题目 #${item.problemId}` }}</h3>
              </div>
              <span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span>
            </div>
            <div class="submission-meta">
              <span class="pill">{{ item.language.toUpperCase() }}</span>
              <span>{{ formatTime(item.createdAt) }}</span>
            </div>
          </router-link>
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

      <section v-else class="empty-state">
        <strong>你还没有提交记录</strong>
        <span class="muted">从题库里选一道题，提交第一份代码吧。</span>
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
.page-title-block {
  padding: 24px 28px;
}

.hero-mini {
  display: grid;
  justify-items: end;
  gap: 4px;
}

.hero-mini strong {
  font-size: 34px;
  letter-spacing: -0.05em;
}

.submissions-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.submission-list {
  display: grid;
  gap: 16px;
}

.submission-card {
  display: grid;
  gap: 16px;
  padding: 22px;
  border-radius: 24px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.68);
  box-shadow: var(--shadow-sm);
  transition: transform var(--transition), box-shadow var(--transition);
}

.submission-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-md);
}

.submission-main,
.submission-meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.submission-id {
  display: inline-block;
  font-size: 12px;
  font-weight: 700;
  color: var(--brand-deep);
  margin-bottom: 6px;
}

.submission-card h3 {
  margin: 0;
  font-size: 20px;
}

.submission-meta {
  color: var(--ink-soft);
  font-size: 14px;
}

.status-pill {
  padding: 8px 12px;
  border-radius: 999px;
  font-weight: 700;
  background: rgba(60, 116, 198, 0.14);
}

.status-pill.status-AC {
  background: rgba(31, 141, 96, 0.14);
}

.status-pill.status-WA,
.status-pill.status-RE {
  background: rgba(187, 77, 58, 0.14);
}

@media (max-width: 640px) {
  .submissions-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero-mini {
    justify-items: start;
  }
}
</style>
