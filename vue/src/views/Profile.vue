<template>
  <div class="page">
    <section class="page-hero profile-hero">
      <div class="profile-intro">
        <span class="eyebrow">Profile</span>
        <div class="page-title">
          <div>
            <h1>{{ profile?.username || store.username || 'User' }}</h1>
            <p class="page-subtitle">杩欓噷闆嗕腑灞曠ず璐﹀彿鐢诲儚銆佸凡閫氳繃棰樼洰銆丄I 璁粌鍏ュ彛锛屼互鍙婄鐞嗗憳鐨勫悗鍙板伐浣滃彴鍏ュ彛銆?/p>
          </div>
        </div>
      </div>

      <div class="profile-badge-panel">
        <span class="profile-avatar">{{ (profile?.username || store.username || 'G').slice(0, 1).toUpperCase() }}</span>
        <span class="badge" :class="store.isAdmin ? 'badge-admin' : 'badge-success'">
          {{ store.isAdmin ? '绠＄悊鍛? : '鏅€氱敤鎴? }}
        </span>
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>涓汉鏁版嵁鍔犺浇涓?/strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else-if="profile">
      <section class="metric-grid">
        <article class="metric-card">
          <span class="metric-value">{{ profile.solvedCount }}</span>
          <span class="metric-label">宸茶В鍐抽鐩?/span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ profile.solvedList.length }}</span>
          <span class="metric-label">AC 璁板綍鏁?/span>
        </article>
        <article class="metric-card">
          <span class="metric-value">{{ profile.role === 1 ? 'Admin' : 'User' }}</span>
          <span class="metric-label">褰撳墠瑙掕壊</span>
        </article>
      </section>

      <section class="profile-grid">
        <article class="card stack">
          <div class="section-title">
            <h2>蹇嵎鍏ュ彛</h2>
          </div>
          <div class="quick-links">
            <router-link to="/chat" class="quick-link feature-link">
              <strong>AI 璁粌璁″垝</strong>
              <span>鍩轰簬鍘嗗彶鎻愪氦鍜屾爣绛剧敾鍍忕敓鎴愭帹鑽愰鍗?/span>
            </router-link>
            <router-link to="/my-submissions" class="quick-link">鏌ョ湅鎻愪氦璁板綍</router-link>
            <router-link to="/leaderboard" class="quick-link">杩涘叆鎺掕姒?/router-link>
            <router-link to="/" class="quick-link">杩斿洖棰樺簱</router-link>
          </div>
          <button class="btn btn-ghost" @click="logout">閫€鍑虹櫥褰?/button>
        </article>

        <article class="card stack">
          <div class="section-title">
            <h2>宸查€氳繃棰樼洰</h2>
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
            <strong>杩樻病鏈?AC 璁板綍</strong>
            <span class="muted">鍏堝畬鎴愬嚑閬撻锛屽啀鍥炴潵鐢?AI 璁粌璁″垝鍋氭洿鏈夐拡瀵规€х殑鎺ㄨ崘銆?/span>
          </div>
        </article>
      </section>

      <section class="ai-panel">
        <div>
          <span class="eyebrow">Agent Workflow</span>
          <h2>鎺ㄨ崘绯荤粺宸茬粡鎺ュ埌鍓嶅彴浜嗐€?/h2>
          <p>鐜板湪浣犲彲浠ヤ粠鍓嶇鐩存帴瑙﹀彂 `study_plan` 寮傛浠诲姟锛屾煡鐪嬪急椤规爣绛俱€佹帹鑽愰鍗曚笌鎬荤粨寤鸿锛岃€屼笉鏄彧鍦?README 閲屽睍绀鸿繖鏉¤兘鍔涖€?/p>
        </div>
        <div class="cluster">
          <router-link to="/chat" class="btn btn-secondary">鎵撳紑 AI 璁粌璁″垝</router-link>
        </div>
      </section>

      <section v-if="store.isAdmin" class="admin-panel">
        <div>
          <span class="eyebrow">Admin Console</span>
          <h2>绠＄悊宸ヤ綔鍖?/h2>
          <p>绠＄悊鍛樺彲浠ョ洿鎺ュ湪杩欓噷杩涘叆鐢ㄦ埛灏佺绠＄悊銆侀鐩鐞嗐€佹爣绛剧鐞嗗拰鏂板缓棰樼洰鍏ュ彛锛岀敤浜庡睍绀哄悗鍙拌繍钀ヨ兘鍔涖€?/p>
        </div>
        <div class="cluster">
          <router-link to="/admin/users" class="btn btn-ghost">鐢ㄦ埛绠＄悊</router-link>
          <router-link to="/admin/problems" class="btn btn-outline">棰樼洰绠＄悊</router-link>
          <router-link to="/admin/problems/new" class="btn btn-primary">鏂板缓棰樼洰</router-link>
          <router-link to="/admin/tags" class="btn btn-secondary">鏍囩绠＄悊</router-link>
        </div>
      </section>
    </template>

    <section v-else class="empty-state">
      <strong>娌℃湁鎷垮埌涓汉淇℃伅</strong>
      <span class="muted">鍙互閲嶆柊鐧诲綍鍚庡啀璇曚竴娆°€?/span>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getProfile, logoutUser } from '../api'
import { store } from '../store'

const router = useRouter()
const loading = ref(true)
const profile = ref(null)

async function logout() {
  await logoutUser()
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
