<template>
  <div class="page">
    <section class="page-hero leaderboard-hero">
      <div>
        <span class="eyebrow">Leaderboard</span>
        <div class="page-title">
          <div>
            <h1>算法荣誉榜</h1>
            <p class="page-subtitle">每一道通过的题目，都是积累的实力。与大家一起，在练习中不断进阶。</p>
          </div>
        </div>
      </div>
      <div class="hero-rank">
        <strong v-if="store.isLoggedIn && myRank > 0">当前排名：第 {{ myRank }} 名</strong>
        <strong v-else-if="store.isLoggedIn">当前还没有进入榜单</strong>
        <strong v-else>登录后可查看你的个人排名</strong>
        <span v-if="store.isLoggedIn">积分：{{ myScore }}</span>
      </div>
    </section>

    <section v-if="loading" class="loading-state">
      <strong>排行榜加载中</strong>
      <span class="spinner spinner-dark"></span>
    </section>

    <section v-else-if="error" class="empty-state" role="alert"><strong>排行榜暂时无法加载</strong><span class="muted">{{ error }}</span><button class="btn btn-outline" @click="loadBoard">重试</button></section>
    <template v-else>
      <section v-if="podium.length" class="podium-grid">
        <article v-for="entry in podium" :key="entry.userId" class="podium-card" :class="`rank-${entry.rank}`">
          <span class="podium-rank">TOP {{ entry.rank }}</span>
          <span class="podium-avatar">{{ entry.username.slice(0, 1).toUpperCase() }}</span>
          <strong>{{ entry.username }}</strong>
          <span>{{ entry.score }} pts</span>
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
        <strong>榜单暂时为空</strong>
        <span class="muted">完成你的第一道题，让努力出现在这里。</span>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getLeaderboard, getErrorMessage } from '../api'
import { store } from '../store'

const loading = ref(true)
const error = ref('')
const top50 = ref([])
const myRank = ref(-1)
const myScore = ref(0)

const podium = computed(() => top50.value.slice(0, 3))
const restOfBoard = computed(() => top50.value.slice(3))

async function loadBoard() {
  loading.value = true
  error.value = ''
  try {
    const data = await getLeaderboard()
    top50.value = data.top50
    myRank.value = data.myRank
    myScore.value = data.myScore
  } catch (requestError) {
    error.value = getErrorMessage(requestError, '请检查网络连接后重试。')
  } finally {
    loading.value = false
  }
}
onMounted(loadBoard)
</script>

<style scoped>
.leaderboard-hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
  background: #142440;
  border-color: #243957;
  color: white;
  padding: 34px;
}

.leaderboard-hero .eyebrow {
  color: #a9bcdf;
}

.leaderboard-hero .page-subtitle {
  color: #9fafc9;
  max-width: 560px;
}

.hero-rank {
  display: grid;
  gap: 6px;
  text-align: right;
  padding: 18px 22px;
  border-radius: 10px;
  background: #ffffff08;
  border: 1px solid #ffffff12;
}

.hero-rank strong {
  font-size: 14px;
  font-weight: 600;
}

.hero-rank span {
  font-size: 12px;
  color: #b3efcd;
}

.podium-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
}

.podium-card {
  display: grid;
  gap: 9px;
  justify-items: center;
  text-align: center;
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  background: white;
  position: relative;
  overflow: hidden;
}

.podium-card::before {
  content: '';
  position: absolute;
  height: 3px;
  top: 0;
  left: 0;
  right: 0;
  background: #c7d0e0;
}

.podium-card.rank-1 {
  background: linear-gradient(180deg, #fffcf3, white);
  border-color: #eee3c6;
}

.podium-card.rank-1::before {
  background: #d6b366;
}

.podium-card.rank-3::before {
  background: #c99c89;
}

.podium-rank {
  font-size: 9px;
  font-weight: 700;
  color: #8a97ad;
  letter-spacing: 2px;
}

.rank-1 .podium-rank {
  color: #b78d39;
}

.podium-avatar {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: 50%;
  background: #eff3fb;
  color: #7284a5;
  font-size: 20px;
  font-weight: 650;
  margin: 6px 0;
}

.rank-1 .podium-avatar {
  background: #f4e9cb;
  color: #b08735;
}

.rank-3 .podium-avatar {
  background: #f9efea;
  color: #b1826d;
}

.podium-card > strong {
  font-size: 16px;
  overflow-wrap: anywhere;
  max-width: 100%;
}

.podium-card > span:last-child {
  color: var(--ink-faint);
  font: 12px Consolas, monospace;
}

.board-table {
  padding: 0;
  overflow: hidden;
}

.board-row {
  display: grid;
  grid-template-columns: 100px minmax(0, 1fr) 100px;
  gap: 16px;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--line);
}

.board-row:last-child {
  border-bottom: 0;
}

.board-head {
  font-size: 11px;
  color: var(--ink-faint);
  background: #f9fafd;
}

.board-row.current {
  background: var(--surface-tint);
}

.rank-col {
  font: 12px Consolas, monospace;
  color: var(--ink-faint);
}

.user-col {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.user-col strong {
  overflow-wrap: anywhere;
}

.user-badge {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: #f0f3fa;
  color: #788ba9;
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.score-col {
  text-align: right;
  font: 600 13px Consolas, monospace;
}

.board-head > span:last-child {
  text-align: right;
}

@media (max-width: 760px) {
  .leaderboard-hero {
    flex-direction: column;
    align-items: flex-start;
    padding: 24px;
  }
  .hero-rank {
    text-align: left;
    width: 100%;
  }
  .podium-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .podium-card {
    display: flex;
    text-align: left;
    padding: 16px;
    gap: 14px;
  }
  .podium-avatar {
    width: 38px;
    height: 38px;
    margin: 0;
    flex-shrink: 0;
    font-size: 16px;
  }
  .podium-rank {
    letter-spacing: 1px;
    white-space: nowrap;
  }
  .podium-card > strong {
    flex: 1;
    font-size: 14px;
  }
  .podium-card > span:last-child {
    flex-shrink: 0;
  }
  .board-row {
    grid-template-columns: 50px minmax(0, 1fr) 50px;
    gap: 10px;
    padding: 14px 16px;
  }
}
</style>