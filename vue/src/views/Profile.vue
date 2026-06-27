<template>
  <div class="page">
    <section class="page-hero profile-hero">
      <div class="profile-intro">
        <span class="eyebrow">Profile</span>
        <div class="page-title">
          <div>
            <h1>{{ profile?.username || store.username || 'User' }}</h1>
            <p class="page-subtitle">这里集中展示账号画像、已通过题目、AI 训练入口，以及管理员的后台工作台入口。</p>
          </div>
        </div>
      </div>

      <div class="profile-badge-panel">
        <span class="profile-avatar">{{ (profile?.username || store.username || 'G').slice(0, 1).toUpperCase() }}</span>
        <span class="badge" :class="store.isAdmin ? 'badge-admin' : 'badge-success'">
          {{ store.isAdmin ? '管理员' : '普通用户' }}
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
          <span class="metric-label">已解决题目</span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ profile.solvedList.length }}</span>
          <span class="metric-label">AC 记录数</span>
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
            <router-link to="/study-plan" class="quick-link feature-link">
              <strong>AI 训练计划</strong>
              <span>基于历史提交和标签画像生成推荐题单</span>
            </router-link>
            <router-link to="/my-submissions" class="quick-link">查看提交记录</router-link>
            <router-link to="/leaderboard" class="quick-link">进入排行榜</router-link>
            <router-link to="/" class="quick-link">返回题库</router-link>
          </div>
          <button class="btn btn-ghost" @click="logout">退出登录</button>
        </article>

        <article class="card stack">
          <div class="section-title">
            <h2>已通过题目</h2>
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
          <div v-else class="empty-state compact-state">
            <strong>还没有 AC 记录</strong>
            <span class="muted">先完成几道题，再回来用 AI 训练计划做更有针对性的推荐。</span>
          </div>
        </article>
      </section>

      <section class="ai-panel">
        <div>
          <span class="eyebrow">Agent Workflow</span>
          <h2>推荐系统已经接到前台了。</h2>
          <p>现在你可以从前端直接触发 `study_plan` 异步任务，查看弱项标签、推荐题单与总结建议，而不是只在 README 里展示这条能力。</p>
        </div>
        <div class="cluster">
          <router-link to="/study-plan" class="btn btn-secondary">打开 AI 训练计划</router-link>
        </div>
      </section>

      <section v-if="store.isAdmin" class="admin-panel">
        <div>
          <span class="eyebrow">Admin Console</span>
          <h2>管理工作区</h2>
          <p>管理员可以直接在这里进入题目管理、标签管理和新建题目入口，用于展示后台运营能力。</p>
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
      <span class="muted">可以重新登录后再试一次。</span>
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
  background: linear-gradient(135deg, var(--brand), var(--accent));
  color: #f8fbff;
  font-size: 34px;
  font-weight: 800;
  box-shadow: 0 20px 34px rgba(37, 99, 235, 0.18);
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
  display: grid;
  gap: 4px;
  padding: 14px 16px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.7);
  border: 1px solid var(--line);
  font-weight: 700;
}

.feature-link {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.08), rgba(15, 118, 110, 0.08));
}

.quick-link span {
  font-size: 13px;
  color: var(--ink-soft);
  font-weight: 600;
}

.quick-link:hover,
.solved-pill:hover {
  transform: translateY(-2px);
}

.solved-pill {
  background: rgba(15, 118, 110, 0.1);
  color: var(--accent-deep);
}

.compact-state {
  padding: 26px 16px;
}

.ai-panel,
.admin-panel {
  display: grid;
  gap: 18px;
  padding: 28px;
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
}

.ai-panel {
  background: linear-gradient(135deg, rgba(15, 118, 110, 0.12), rgba(255, 255, 255, 0.72));
  border: 1px solid rgba(15, 118, 110, 0.12);
}

.admin-panel {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.12), rgba(255, 255, 255, 0.72));
  border: 1px solid rgba(37, 99, 235, 0.12);
}

.ai-panel h2,
.admin-panel h2 {
  margin: 16px 0 8px;
  font-size: 34px;
  letter-spacing: -0.04em;
}

.ai-panel p,
.admin-panel p {
  margin: 0;
  max-width: 680px;
  color: var(--ink-soft);
}

@media (max-width: 820px) {
  .profile-grid {
    grid-template-columns: 1fr;
  }

  .profile-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
