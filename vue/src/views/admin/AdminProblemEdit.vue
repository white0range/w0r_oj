<template>
  <div class="page">
    <section class="page-hero admin-hero">
      <div>
        <span class="eyebrow">{{ isNew ? 'Create Problem' : 'Edit Problem' }}</span>
        <div class="page-title">
          <div>
            <h1>{{ isNew ? '新建题目' : `编辑题目 #${problemId}` }}</h1>
            <p class="page-subtitle">
              新建会直接提交 `title / description / tag_ids / test_cases`；编辑则拆成基础信息、标签关联和测试用例增量添加，对齐当前后端实现。
            </p>
          </div>
        </div>
      </div>
      <router-link to="/admin/problems" class="btn btn-outline">返回题目管理</router-link>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>表单数据加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <form v-else class="stack" @submit.prevent="handleSubmit">
      <section class="card stack">
        <div class="section-title">
          <h2>基础信息</h2>
        </div>
        <div class="field">
          <label for="title">标题</label>
          <input id="title" v-model.trim="form.title" class="input" required />
        </div>
        <div class="form-grid">
          <div class="field">
            <label for="time-limit">时间限制（ms）</label>
            <input id="time-limit" v-model.number="form.time_limit" class="input" type="number" min="1" />
          </div>
          <div class="field">
            <label for="memory-limit">内存限制（MB）</label>
            <input id="memory-limit" v-model.number="form.memory_limit" class="input" type="number" min="1" />
          </div>
        </div>
        <div class="field">
          <label for="description">题目描述</label>
          <textarea id="description" v-model="form.description" class="textarea" required></textarea>
        </div>
      </section>

      <section class="card stack">
        <div class="section-title">
          <h2>标签关联</h2>
        </div>
        <div class="cluster">
          <label v-for="tag in tags" :key="tag.id" class="check-chip">
            <input v-model="form.tag_ids" type="checkbox" :value="tag.id" />
            <span>{{ tag.name }}</span>
          </label>
        </div>
        <p v-if="!tags.length" class="helper-text">还没有标签，可以先去标签管理页创建。</p>
      </section>

      <section class="card stack">
        <div class="section-title">
          <h2>新增测试用例</h2>
          <button class="btn btn-outline btn-sm" type="button" @click="addTestCase">添加一组用例</button>
        </div>

        <div v-if="!form.test_cases.length" class="empty-state compact">
          <strong>暂时没有新增测试用例</strong>
          <span class="muted">新建题目建议至少带一组测试用例；编辑已有题目时也可以按需追加。</span>
        </div>

        <div v-for="(testCase, index) in form.test_cases" :key="index" class="case-card">
          <div class="section-title">
            <h3>新增用例 {{ index + 1 }}</h3>
            <button class="btn btn-danger btn-sm" type="button" @click="removeTestCase(index)">删除</button>
          </div>
          <div class="form-grid">
            <div class="field">
              <label>输入</label>
              <textarea v-model="testCase.input" class="textarea mono small-textarea" required></textarea>
            </div>
            <div class="field">
              <label>期望输出</label>
              <textarea v-model="testCase.expected_output" class="textarea mono small-textarea" required></textarea>
            </div>
          </div>
        </div>
      </section>

      <section v-if="!isNew && existingCases.length" class="card stack">
        <div class="section-title">
          <h2>已有测试用例</h2>
        </div>
        <div v-for="item in existingCases" :key="item.id" class="existing-case">
          <div class="section-title">
            <h3>用例 #{{ item.id }}</h3>
            <button class="btn btn-danger btn-sm" type="button" @click="deleteCase(item.id)">删除</button>
          </div>
          <div class="form-grid">
            <div class="field">
              <label>输入</label>
              <pre class="case-preview mono">{{ item.input }}</pre>
            </div>
            <div class="field">
              <label>期望输出</label>
              <pre class="case-preview mono">{{ item.expectedOutput }}</pre>
            </div>
          </div>
        </div>
      </section>

      <div v-if="error" class="auth-message auth-error">{{ error }}</div>
      <div v-if="success" class="auth-message auth-success">{{ success }}</div>

      <div class="cluster">
        <button class="btn btn-primary" :disabled="saving" type="submit">
          <span v-if="saving" class="spinner"></span>
          <span v-else>{{ isNew ? '创建题目' : '保存修改' }}</span>
        </button>
        <router-link to="/admin/problems" class="btn btn-ghost">取消</router-link>
      </div>
    </form>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  adminAddTestCase,
  adminCreateProblem,
  adminDeleteTestCase,
  adminGetTestCases,
  adminUpdateProblem,
  adminUpdateProblemTags,
  getProblemDetail,
  getTags,
} from '../../api'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const success = ref('')
const tags = ref([])
const existingCases = ref([])

const form = reactive({
  title: '',
  description: '',
  time_limit: 1000,
  memory_limit: 256,
  tag_ids: [],
  test_cases: [],
})

const isNew = computed(() => route.name === 'admin-problem-new')
const problemId = computed(() => Number(route.params.id || 0))

function addTestCase() {
  form.test_cases.push({
    input: '',
    expected_output: '',
  })
}

function removeTestCase(index) {
  form.test_cases.splice(index, 1)
}

async function loadTags() {
  tags.value = await getTags()
}

async function loadProblem() {
  if (isNew.value) {
    return
  }

  const [detail, cases] = await Promise.all([getProblemDetail(problemId.value), adminGetTestCases(problemId.value)])

  form.title = detail.title
  form.description = detail.description
  form.time_limit = detail.timeLimit
  form.memory_limit = detail.memoryLimit
  form.tag_ids = detail.tags.map((tag) => tag.id)
  existingCases.value = cases.items
}

async function deleteCase(caseId) {
  await adminDeleteTestCase(caseId)
  existingCases.value = existingCases.value.filter((item) => item.id !== caseId)
}

async function handleSubmit() {
  saving.value = true
  error.value = ''
  success.value = ''

  try {
    const payload = {
      title: form.title,
      description: form.description,
      time_limit: form.time_limit,
      memory_limit: form.memory_limit,
      tag_ids: form.tag_ids,
      test_cases: form.test_cases,
    }

    if (isNew.value) {
      await adminCreateProblem(payload)
      success.value = '题目已创建，正在返回管理页。'
      setTimeout(() => router.push('/admin/problems'), 700)
      return
    }

    await adminUpdateProblem(problemId.value, payload)
    await adminUpdateProblemTags(problemId.value, { tag_ids: form.tag_ids })

    for (const testCase of form.test_cases) {
      await adminAddTestCase(problemId.value, testCase)
    }

    success.value = '题目已更新。'
    form.test_cases = []
    existingCases.value = (await adminGetTestCases(problemId.value)).items
  } catch (requestError) {
    error.value = requestError.response?.data?.error || '保存失败，请检查后端返回。'
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    await Promise.all([loadTags(), loadProblem()])
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.admin-hero {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 18px;
}

.form-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.check-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.62);
  font-weight: 700;
}

.case-card,
.existing-case {
  padding: 18px;
  border-radius: 22px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.58);
}

.small-textarea {
  min-height: 140px;
}

.case-preview {
  margin: 0;
  padding: 14px;
  border-radius: 18px;
  background: var(--surface-dark);
  color: #eef4ff;
  overflow: auto;
  white-space: pre-wrap;
}

@media (max-width: 820px) {
  .admin-hero,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .admin-hero {
    flex-direction: column;
    align-items: start;
  }
}
</style>
