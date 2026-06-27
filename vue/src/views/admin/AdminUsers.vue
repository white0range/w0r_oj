<template>
  <div class="page">
    <section class="page-hero admin-hero">
      <div>
        <span class="eyebrow">Admin Console</span>
        <div class="page-title">
          <div>
            <h1>用户管理</h1>
            <p class="page-subtitle">
              这里可以查看账号状态，并对普通用户执行封禁或解封。封禁后旧 access token 和 refresh token 都会失效。
            </p>
          </div>
        </div>
      </div>

      <div class="cluster">
        <router-link to="/admin/problems" class="btn btn-outline">题目管理</router-link>
        <router-link to="/admin/tags" class="btn btn-ghost">标签管理</router-link>
      </div>
    </section>

    <section class="card stack">
      <div class="toolbar">
        <input
          v-model.trim="keyword"
          class="input"
          placeholder="按用户名搜索"
          @keyup.enter="loadUsers"
        />
        <button class="btn btn-primary" :disabled="loading" @click="loadUsers">
          <span v-if="loading" class="spinner"></span>
          <span v-else>刷新列表</span>
        </button>
      </div>

      <div v-if="message" class="auth-message" :class="messageType === 'error' ? 'auth-error' : 'auth-success'">
        {{ message }}
      </div>

      <section v-if="users.length" class="card admin-table">
        <div class="admin-row admin-head user-row">
          <span>用户</span>
          <span>角色</span>
          <span>状态</span>
          <span>通过数</span>
          <span>封禁原因</span>
          <span>操作</span>
        </div>

        <div v-for="user in users" :key="user.id" class="admin-row user-row">
          <div class="stack-xs">
            <strong>{{ user.username }}</strong>
            <small class="muted">#{{ user.id }}</small>
          </div>
          <span>{{ user.role === 1 ? 'Admin' : 'User' }}</span>
          <span>
            <span class="badge" :class="user.status === 'banned' ? 'badge-warning' : 'badge-success'">
              {{ user.status === 'banned' ? '已封禁' : '正常' }}
            </span>
          </span>
          <strong>{{ user.solved_count }}</strong>
          <span class="muted">{{ user.ban_reason || '-' }}</span>
          <div class="cluster">
            <button
              v-if="user.role !== 1 && user.status !== 'banned'"
              class="btn btn-sm btn-outline"
              :disabled="actingUserId === user.id"
              @click="banUser(user)"
            >
              封禁
            </button>
            <button
              v-if="user.role !== 1 && user.status === 'banned'"
              class="btn btn-sm btn-secondary"
              :disabled="actingUserId === user.id"
              @click="unbanUser(user)"
            >
              解封
            </button>
          </div>
        </div>
      </section>

      <section v-else class="empty-state compact-state">
        <strong>没有查到用户</strong>
        <span class="muted">可以调整搜索关键字后再试一次。</span>
      </section>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { adminBanUser, adminGetUsers, adminUnbanUser, getErrorMessage } from '../../api'

const users = ref([])
const loading = ref(false)
const actingUserId = ref(0)
const keyword = ref('')
const message = ref('')
const messageType = ref('success')

async function loadUsers() {
  loading.value = true

  try {
    users.value = await adminGetUsers({ keyword: keyword.value, limit: 100 })
    if (!message.value || messageType.value !== 'success') {
      message.value = ''
    }
  } catch (error) {
    message.value = getErrorMessage(error, '读取用户列表失败。')
    messageType.value = 'error'
  } finally {
    loading.value = false
  }
}

async function banUser(user) {
  const reason = window.prompt(`请输入封禁 ${user.username} 的原因`, '违反平台规则')
  if (reason === null) {
    return
  }

  actingUserId.value = user.id
  message.value = ''

  try {
    await adminBanUser(user.id, { reason })
    message.value = `已封禁用户 ${user.username}`
    messageType.value = 'success'
    await loadUsers()
  } catch (error) {
    message.value = getErrorMessage(error, '封禁用户失败。')
    messageType.value = 'error'
  } finally {
    actingUserId.value = 0
  }
}

async function unbanUser(user) {
  actingUserId.value = user.id
  message.value = ''

  try {
    await adminUnbanUser(user.id)
    message.value = `已解封用户 ${user.username}`
    messageType.value = 'success'
    await loadUsers()
  } catch (error) {
    message.value = getErrorMessage(error, '解封用户失败。')
    messageType.value = 'error'
  } finally {
    actingUserId.value = 0
  }
}

onMounted(loadUsers)
</script>

<style scoped>
.admin-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: center;
}

.toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
}

.toolbar .input {
  max-width: 280px;
}

.admin-table {
  padding: 0;
}

.admin-row {
  display: grid;
  gap: 12px;
  align-items: center;
  padding: 16px 18px;
  border-bottom: 1px solid var(--line);
}

.user-row {
  grid-template-columns: 1.2fr 0.7fr 0.8fr 0.6fr 1.2fr 0.8fr;
}

.admin-head {
  background: rgba(15, 23, 42, 0.04);
  font-size: 12px;
  font-weight: 800;
  color: var(--ink-faint);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.stack-xs {
  display: grid;
  gap: 4px;
}

.compact-state {
  padding: 24px 16px;
}

@media (max-width: 980px) {
  .admin-hero,
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar .input {
    max-width: none;
    width: 100%;
  }

  .user-row {
    grid-template-columns: 1fr;
  }

  .admin-head {
    display: none;
  }
}
</style>
