<template>
  <div class="page">
    <section class="page-hero profile-hero">
      <div class="profile-intro">
        <span class="eyebrow">Profile</span>
        <div class="page-title">
          <div>
            <h1>{{ profile?.username || store.username || '开发者' }}</h1>
            <p class="page-subtitle">个人中心直接对接 `/api/profile`，同步用户角色和已解题状态，为前端路由守卫和后台入口提供依据。</p>
          </div>
        </div>
      </div>

      <div class="profile-badge-panel">
        <span class="profile-avatar">{{ (profile?.username || store.username || 'G').slice(0, 1).toUpperCase() }}</span>
        <span class="badge" :class="store.isAdmin ? 'badge-admin' : 'badge-success'">
          {{ store.isAdmin ? '管理员账号' : '普通用户' }}
        </span>
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>个人数据加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else-if="profile">
      <section class="metric-grid">
        <article class="metric-card">
          <span class="metric-value">{{ profile.solvedCount }}</span>
          <span class="metric-label">已解题数</span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ profile.solvedList.length }}</span>
          <span class="metric-label">已记录 AC 题目</span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ profile.role === 1 ? 'Admin' : 'User' }}</span>
          <span class="metric-label">当前角色</span>
        </article>
      </section>

      <section class="profile-grid">
        <article class="card stack">
          <div class="section-title">
            <h2>快捷入口</h2>
          </div>
          <div class="quick-links">
            <router-link to="/my-submissions" class="quick-link">查看提交记录</router-link>
            <router-link to="/leaderboard" class="quick-link">进入排行榜</router-link>
            <router-link to="/" class="quick-link">返回题库</router-link>
          </div>
          <button class="btn btn-ghost" @click="logout">退出登录</button>
        </article>

        <article class="card stack">
          <div class="section-title">
            <h2>已解题目</h2>
            <span class="muted">{{ profile.solvedList.length }} items</span>
          </div>
          <div v-if="profile.solvedList.length" class="cluster">
            <router-link
              v-for="problemId in profile.solvedList"
              :key="problemId"
              :to="`/problems/${problemId}`"
              class="pill solved-pill"
            >
              #{{ problemId }}
            </router-link>
          </div>
          <div v-else class="empty-state compact">
            <strong>还没有 AC 记录</strong>
            <span class="muted">先去提交一道题，个人中心就会开始累计数据。</span>
          </div>
        </article>
      </section>

      <section v-if="store.isAdmin" class="admin-panel">
        <div>
          <span class="eyebrow">Admin Console</span>
          <h2>管理员工作区</h2>
          <p>这里汇总了题目管理、标签管理和新建题目入口，方便你直接演示后台能力以及前后端协作流程。</p>
        </div>
        <div class="cluster">
          <router-link to="/admin/problems" class="btn btn-outline">题目管理</router-link>
          <router-link to="/admin/problems/new" class="btn btn-primary">新建题目</router-link>
          <router-link to="/admin/tags" class="btn btn-secondary">标签管理</router-link>
        </div>
      </section>
    </template>

    <section v-else class="empty-state">
      <strong>没有拿到个人信息</strong>
      <span class="muted">可以重新登录一次再看看。</span>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getProfile } from '../api'
import { store } from '../store'

const router = useRouter()
const loading = ref(true)
const profile = ref(null)

function logout() {
  store.logout()
  router.push('/')
}

onMounted(async () => {
  try {
    const data = await getProfile()
    profile.value = data
    store.hydrateProfile(data)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.profile-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: center;
}

.profile-badge-panel {
  display: grid;
  justify-items: center;
  gap: 12px;
}

.profile-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--brand), var(--brand-deep));
  color: #fff9f5;
  font-size: 34px;
  font-weight: 700;
  box-shadow: 0 20px 34px rgba(153, 57, 28, 0.22);
}

.profile-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.quick-links {
  display: grid;
  gap: 10px;
}

.quick-link,
.solved-pill {
  transition: all var(--transition);
}

.quick-link {
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.62);
  border: 1px solid var(--line);
  font-weight: 700;
}

.quick-link:hover,
.solved-pill:hover {
  transform: translateY(-2px);
}

.solved-pill {
  background: rgba(15, 139, 131, 0.12);
  color: var(--accent);
}

.admin-panel {
  display: grid;
  gap: 18px;
  padding: 28px;
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, rgba(209, 98, 57, 0.16), rgba(255, 255, 255, 0.72));
  border: 1px solid rgba(209, 98, 57, 0.16);
  box-shadow: var(--shadow-md);
}

.admin-panel h2 {
  margin: 16px 0 8px;
  font-size: 34px;
  letter-spacing: -0.04em;
}

.admin-panel p {
  margin: 0;
  max-width: 680px;
  color: var(--ink-soft);
}

.compact {
  padding: 24px 16px;
}

@media (max-width: 820px) {
  .profile-hero,
  .profile-grid {
    grid-template-columns: 1fr;
  }

  .profile-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
