<template>
  <div class="page">
    <section class="page-hero leaderboard-hero">
      <div>
        <span class="eyebrow">Leaderboard</span>
        <div class="page-title">
          <div>
            <h1>排行榜</h1>
            <p class="page-subtitle">这里严格对接 `/api/leaderboard`，支持游客查看榜单，登录用户查看自己的排名。</p>
          </div>
        </div>
      </div>
      <div class="hero-rank">
        <strong v-if="store.isLoggedIn && myRank > 0">你的当前排名：第 {{ myRank }} 名</strong>
        <strong v-else-if="store.isLoggedIn">你还没有上榜，去刷题吧。</strong>
        <strong v-else>登录后可查看你的专属排名。</strong>
        <span v-if="store.isLoggedIn">积分：{{ myScore }}</span>
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>排行榜载入中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <template v-else>
      <section v-if="podium.length" class="podium-grid">
        <article
          v-for="entry in podium"
          :key="entry.userId"
          class="podium-card"
          :class="`rank-${entry.rank}`"
        >
          <span class="podium-rank">#{{ entry.rank }}</span>
          <span class="podium-avatar">{{ entry.username.slice(0, 1).toUpperCase() }}</span>
          <strong>{{ entry.username }}</strong>
          <span>{{ entry.score }} 分</span>
        </article>
      </section>

      <section v-if="restOfBoard.length" class="card board-table">
        <div class="board-row board-head">
          <span>排名</span>
          <span>用户</span>
          <span>积分</span>
        </div>
        <div
          v-for="entry in restOfBoard"
          :key="entry.userId"
          class="board-row"
          :class="{ current: entry.userId === store.userId }"
        >
          <span class="rank-col">#{{ entry.rank }}</span>
          <span class="user-col">
            <span class="user-badge">{{ entry.username.slice(0, 1).toUpperCase() }}</span>
            <strong>{{ entry.username }}</strong>
            <span v-if="entry.userId === store.userId" class="badge badge-admin">我</span>
          </span>
          <span class="score-col">{{ entry.score }}</span>
        </div>
      </section>

      <section v-if="!top50.length" class="empty-state">
        <strong>暂时还没有榜单数据</strong>
        <span class="muted">先去提交几次代码，让这个小型 OJ 真正动起来。</span>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getLeaderboard } from '../api'
import { store } from '../store'

const loading = ref(true)
const top50 = ref([])
const myRank = ref(-1)
const myScore = ref(0)

const podium = computed(() => top50.value.slice(0, 3))
const restOfBoard = computed(() => top50.value.slice(3))

onMounted(async () => {
  try {
    const data = await getLeaderboard()
    top50.value = data.top50
    myRank.value = data.myRank
    myScore.value = data.myScore
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.leaderboard-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: end;
  padding: 28px;
}

.hero-rank {
  display: grid;
  gap: 6px;
  justify-items: end;
  text-align: right;
}

.hero-rank strong {
  font-size: 22px;
  letter-spacing: -0.03em;
}

.hero-rank span {
  color: var(--ink-soft);
}

.podium-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
}

.podium-card {
  display: grid;
  gap: 10px;
  justify-items: center;
  text-align: center;
  padding: 24px;
  border-radius: 28px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--line);
}

.podium-card.rank-1 {
  background: linear-gradient(135deg, rgba(203, 109, 67, 0.16), rgba(255, 255, 255, 0.76));
}

.podium-card.rank-2 {
  background: linear-gradient(135deg, rgba(61, 115, 199, 0.14), rgba(255, 255, 255, 0.76));
}

.podium-card.rank-3 {
  background: linear-gradient(135deg, rgba(29, 122, 116, 0.14), rgba(255, 255, 255, 0.76));
}

.podium-rank {
  font-size: 13px;
  font-weight: 800;
  color: var(--brand-deep);
}

.podium-avatar,
.user-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 54px;
  height: 54px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--accent), #115c56);
  color: #f2fffd;
  font-size: 20px;
  font-weight: 800;
}

.board-table {
  padding: 0;
  overflow: hidden;
}

.board-row {
  display: grid;
  grid-template-columns: 110px 1fr 100px;
  gap: 16px;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
}

.board-row:last-child {
  border-bottom: 0;
}

.board-head {
  font-size: 13px;
  font-weight: 800;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.board-row.current {
  background: rgba(203, 109, 67, 0.08);
}

.user-col {
  display: flex;
  align-items: center;
  gap: 12px;
}

.score-col {
  font-weight: 800;
}

@media (max-width: 720px) {
  .leaderboard-hero {
    flex-direction: column;
    align-items: start;
  }

  .hero-rank {
    justify-items: start;
    text-align: left;
  }

  .board-row {
    grid-template-columns: 76px 1fr 72px;
    padding: 14px 16px;
  }
}
</style>
