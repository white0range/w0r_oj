<template>
  <div class="page">
    <section class="page-hero admin-hero">
      <div>
        <span class="eyebrow">Admin Tags</span>
        <div class="page-title">
          <div>
            <h1>标签管理</h1>
            <p class="page-subtitle">新建和删除标签都对接管理员接口，而前台标签展示继续走公开 `/api/tags`。</p>
          </div>
        </div>
      </div>
      <router-link to="/admin/problems" class="btn btn-outline">返回题目管理</router-link>
    </section>

    <section class="card stack">
      <div class="section-title">
        <h2>创建标签</h2>
      </div>
      <form class="cluster tag-form" @submit.prevent="createTag">
        <input v-model.trim="name" class="input" placeholder="例如：动态规划 / 二分 / 图论" required />
        <button class="btn btn-primary" :disabled="creating" type="submit">
          <span v-if="creating" class="spinner"></span>
          <span v-else>创建</span>
        </button>
      </form>
      <div v-if="message" class="auth-message" :class="messageType === 'error' ? 'auth-error' : 'auth-success'">
        {{ message }}
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>标签列表加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <section v-else-if="tags.length" class="tag-grid">
      <article v-for="tag in tags" :key="tag.id" class="tag-card">
        <div>
          <strong>{{ tag.name }}</strong>
          <p>Tag ID #{{ tag.id }}</p>
        </div>
        <button class="btn btn-danger btn-sm" @click="removeTag(tag.id)">删除</button>
      </article>
    </section>

    <section v-else class="empty-state">
      <strong>还没有标签</strong>
      <span class="muted">先创建一个标签，再回到题目编辑页看联动效果。</span>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { adminCreateTag, adminDeleteTag, getTags } from '../../api'

const loading = ref(true)
const creating = ref(false)
const tags = ref([])
const name = ref('')
const message = ref('')
const messageType = ref('success')

async function fetchTags() {
  loading.value = true

  try {
    tags.value = await getTags()
  } finally {
    loading.value = false
  }
}

async function createTag() {
  creating.value = true
  message.value = ''

  try {
    await adminCreateTag({ name: name.value })
    name.value = ''
    message.value = '标签创建成功。'
    messageType.value = 'success'
    await fetchTags()
  } catch (requestError) {
    message.value = requestError.response?.data?.error || '标签创建失败。'
    messageType.value = 'error'
  } finally {
    creating.value = false
  }
}

async function removeTag(tagId) {
  await adminDeleteTag(tagId)
  await fetchTags()
}

onMounted(fetchTags)
</script>

<style scoped>
.admin-hero {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 18px;
}

.tag-form {
  align-items: stretch;
}

.tag-form .input {
  flex: 1;
  min-width: 240px;
}

.tag-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.tag-card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 20px;
  border-radius: 24px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.64);
  box-shadow: var(--shadow-sm);
}

.tag-card strong {
  font-size: 18px;
}

.tag-card p {
  margin: 6px 0 0;
  color: var(--ink-soft);
}

@media (max-width: 820px) {
  .admin-hero {
    flex-direction: column;
    align-items: start;
  }
}
</style>
