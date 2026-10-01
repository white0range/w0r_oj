<template>
  <div class="page workspace-page">
    <div class="workspace-heading"><span><span class="heading-dot"></span>算法学习工作台</span><span class="heading-caption">PRACTICE. THINK. GROW.</span></div>
    <section class="practice-hero">
      <div class="hero-copy">
        <span class="hero-eyebrow">BUILD YOUR NEXT BREAKTHROUGH</span>
        <h1>每一次提交，<br>都是向前一步<span class="hero-period">。</span></h1>
        <p>从一道题开始，在思考与实践中，让解题能力成为你的底气。</p>
        <div class="cluster hero-actions">
          <button class="btn btn-primary" @click="scrollToProblems">开始刷题 <AppIcon name="arrow" /></button>
          <router-link to="/chat" class="hero-text-link"><AppIcon name="sparkles" />和 AI 一起学习 <AppIcon name="chevron" /></router-link>
        </div>
        <div class="hero-topics"><span>算法练习</span><i></i><span>在线判题</span><i></i><span>学习复盘</span></div>
      </div>
      <div class="algorithm-art" aria-hidden="true">
        <div class="art-label"><span class="art-dot"></span>THINK IN ALGORITHMS</div>
        <svg viewBox="0 0 380 205" class="tree-illustration">
          <defs><linearGradient id="tree-line" x1="0" y1="0" x2="1" y2="1"><stop stop-color="#6586dc"/><stop offset="1" stop-color="#a8dac8"/></linearGradient></defs>
          <path d="M190 38 103 108M190 38l87 70M103 108l-51 60M103 108l51 60M277 108l-51 60M277 108l51 60" stroke="url(#tree-line)" stroke-width="1.5" fill="none" />
          <g v-for="node in treeNodes" :key="node.label"><circle :cx="node.x" :cy="node.y" r="22" :fill="node.active ? '#b3efcd' : '#203452'" :stroke="node.active ? '#b3efcd' : '#587093'" stroke-width="1"/><text :x="node.x" :y="node.y + 5" text-anchor="middle" :fill="node.active ? '#143c2b' : '#bccde7'" font-size="14" font-family="Consolas, monospace">{{ node.label }}</text></g>
        </svg>
        <div class="art-code"><span class="code-symbol">↳</span><code>while (learning) { grow(); }</code><span class="art-check"><AppIcon name="check" /></span></div>
        <span class="art-caption">小步迭代 · 不断进阶</span>
      </div>
    </section>

    <section class="overview-grid" aria-label="题库概览">
      <article class="overview-item"><span class="stat-icon"><AppIcon name="book" /></span><div><strong>{{ loading ? '—' : total }}</strong><span>{{ selectedTagId ? '标签内题目' : '可练习题目' }}</span></div><small>PROBLEMS</small></article>
      <article class="overview-item"><span class="stat-icon teal"><AppIcon name="tag" /></span><div><strong>{{ tags.length }}</strong><span>知识点标签</span></div><small>TOPICS</small></article>
      <article class="overview-item"><span class="stat-icon purple"><AppIcon name="code" /></span><div><strong>{{ loading ? '—' : visibleProblems.length }}</strong><span>本页题目</span></div><small>ON THIS PAGE</small></article>
      <article class="overview-item"><span class="stat-icon green"><AppIcon name="check" /></span><div><strong>{{ store.isLoggedIn ? solvedVisibleCount : '—' }}</strong><span>本页已通过</span></div><small>{{ store.isLoggedIn ? 'ACCEPTED' : '登录后记录进度' }}</small></article>
    </section>

    <section id="problem-explorer" class="workspace-grid">
      <section class="problem-explorer card" :aria-busy="loading">
        <div class="explorer-head"><div><h2><AppIcon name="book" />探索题库 <span class="count-badge">{{ total }}</span></h2><p>选择一个知识点，开启今天的练习。</p></div><button class="icon-button" aria-label="刷新题库" :disabled="loading" @click="fetchProblems"><AppIcon name="refresh" /></button></div>
        <div class="search-bar"><AppIcon name="search" /><input id="problem-search" v-model.trim="searchTerm" aria-label="按题号或标题搜索当前页" placeholder="搜索本页题目，输入题号或标题…" /><button v-if="searchTerm" class="icon-button" aria-label="清空搜索" @click="searchTerm = ''"><AppIcon name="close" /></button><span v-else class="search-scope">当前页</span></div>
        <div class="filter-strip" aria-label="题目标签">
          <button class="tag-toggle" :class="{ active: !selectedTagId }" :aria-pressed="!selectedTagId" @click="applyTag(null)">全部题目</button>
          <button v-for="tag in tags" :key="tag.id" class="tag-toggle" :class="{ active: selectedTagId === tag.id }" :aria-pressed="selectedTagId === tag.id" @click="applyTag(tag.id)">{{ tag.name }}</button>
        </div>
        <p v-if="tagsError" class="tags-error" role="status">{{ tagsError }} <button @click="fetchTags">重新加载标签</button></p>
        <div v-if="loading" class="table-skeleton" role="status" aria-label="正在加载题库"><div v-for="index in 6" :key="index" class="skeleton-row"><span></span><span></span><span></span></div></div>
        <div v-else-if="error" class="empty-state" role="alert"><AppIcon name="refresh" /><strong>题库暂时无法加载</strong><span class="muted">{{ error }}</span><button class="btn btn-outline" @click="fetchProblems">重新加载</button></div>
        <template v-else-if="visibleProblems.length">
          <div class="problem-table">
            <div class="problem-row problem-head"><span>状态 / 题号</span><span>题目名称</span><span>知识点</span><span>通过率</span></div>
            <router-link v-for="problem in visibleProblems" :key="problem.id" :to="`/problems/${problem.id}`" class="problem-row problem-item">
              <div class="problem-id"><span class="solve-status" :class="{ solved: problem.isAc }" :title="problem.isAc ? '已通过' : '尚未通过'"><AppIcon :name="problem.isAc ? 'check' : 'circle'" /></span><span class="mono">{{ String(problem.id).padStart(3, '0') }}</span></div>
              <div class="problem-main"><strong>{{ problem.title }}</strong><span>{{ problem.submitCount }} 次提交 <b>·</b> {{ problem.acceptedCount }} 次通过</span></div>
              <div class="problem-tags"><span v-for="tag in problem.tags.slice(0, 2)" :key="tag.id" class="mini-tag">{{ tag.name }}</span><span v-if="problem.tags.length > 2" class="mini-tag" :title="problem.tags.map(tag => tag.name).join('、')">+{{ problem.tags.length - 2 }}</span><span v-if="!problem.tags.length" class="muted">—</span></div>
              <div class="problem-rate"><strong>{{ getAcceptanceRate(problem) }}<small>%</small></strong><span class="rate-track"><i :style="{ width: `${getAcceptanceRate(problem)}%` }"></i></span></div>
            </router-link>
          </div>
        </template>
        <div v-else class="empty-state"><AppIcon name="search" /><strong>没有找到相关题目</strong><span class="muted">试试其他标签，或清空本页搜索。</span><button class="btn btn-outline btn-sm" @click="resetFilters">重置筛选</button></div>
        <div v-if="!error && !loading" class="explorer-footer"><span>第 {{ page }} / {{ totalPages }} 页 · 本页 {{ visibleProblems.length }} 道题</span><div v-if="totalPages > 1" class="pagination"><button class="page-chip" :disabled="page <= 1" aria-label="上一页" @click="changePage(page - 1)">‹</button><button v-for="number in pagesToShow" :key="number" class="page-chip" :class="{ active: number === page }" :aria-current="number === page ? 'page' : undefined" @click="changePage(number)">{{ number }}</button><button class="page-chip" :disabled="page >= totalPages" aria-label="下一页" @click="changePage(page + 1)">›</button></div></div>
      </section>
      <aside class="workspace-sidebar">
        <section class="assistant-card"><span class="assistant-icon"><AppIcon name="sparkles" /></span><span class="sidebar-kicker">YOUR LEARNING PARTNER</span><h3>卡住了？<br>换个思路试试看。</h3><p>拆解算法思路，分析薄弱知识点，找到适合你的下一道题。</p><router-link to="/chat" class="btn btn-primary btn-block">问问 AI 助手 <AppIcon name="arrow" /></router-link><span class="assistant-note">让思考更有方向</span></section>
        <section class="sidebar-card"><div class="sidebar-title"><AppIcon name="trophy" /><h3>在练习中看见成长</h3></div><p>每一次通过都值得记录。看看大家的进度，也为自己定个小目标。</p><router-link to="/leaderboard" class="sidebar-link">查看排行榜 <AppIcon name="arrow" /></router-link></section>
        <section class="practice-note"><span class="note-number">01 / KEEP GOING</span><p>先独立思考，再验证答案。<br>真正的进步，来自每一次复盘。</p><span>Happy coding.</span></section>
      </aside>
    </section>
  </div>
</template>
<script setup>
import { computed, onMounted, ref } from 'vue'
import { getProblems, getTags, getErrorMessage } from '../api'
import { store } from '../store'
import { getAcceptanceRate } from '../utils/normalizers'
import AppIcon from '../components/AppIcon.vue'

const problems = ref([])
const tags = ref([])
const total = ref(0)
const page = ref(1)
const limit = 12
const selectedTagId = ref(null)
const searchTerm = ref('')
const loading = ref(true)
const error = ref('')
const tagsError = ref('')
let requestId = 0
const treeNodes = [
  { x: 190, y: 38, label: '8', active: true },
  { x: 103, y: 108, label: '4', active: true },
  { x: 277, y: 108, label: '12', active: false },
  { x: 52, y: 168, label: '2', active: false },
  { x: 154, y: 168, label: '6', active: true },
  { x: 226, y: 168, label: '10', active: false },
  { x: 328, y: 168, label: '14', active: false },
]
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit)))
const pagesToShow = computed(() => {
  const start = Math.max(1, Math.min(page.value - 2, totalPages.value - 4))
  return Array.from({ length: Math.min(5, totalPages.value) }, (_, index) => start + index)
})
const visibleProblems = computed(() => {
  const keyword = searchTerm.value.toLowerCase()
  return problems.value.filter(problem => !keyword || problem.title.toLowerCase().includes(keyword) || String(problem.id).includes(keyword))
})
const solvedVisibleCount = computed(() => visibleProblems.value.filter(problem => problem.isAc).length)
async function fetchProblems() {
  const currentRequest = ++requestId
  loading.value = true
  error.value = ''
  try {
    const data = await getProblems({ page: page.value, limit, ...(selectedTagId.value ? { tag_id: selectedTagId.value } : {}) })
    if (currentRequest !== requestId) return
    problems.value = data.items
    total.value = data.total
  } catch (requestError) {
    if (currentRequest === requestId) error.value = getErrorMessage(requestError, '请检查网络连接后重试。')
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}
async function fetchTags() {
  tagsError.value = ''
  try { tags.value = await getTags() }
  catch { tagsError.value = '知识点标签加载失败。' }
}
function applyTag(id) { selectedTagId.value = id; page.value = 1; fetchProblems() }
function resetFilters() { searchTerm.value = ''; applyTag(null) }
function changePage(next) {
  if (next < 1 || next > totalPages.value || next === page.value) return
  page.value = next
  fetchProblems()
}
function scrollToProblems() { document.getElementById('problem-explorer')?.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }) }
onMounted(() => Promise.all([fetchProblems(), fetchTags()]))
</script>
<style scoped>
.workspace-page {
  gap: 22px;
}

.workspace-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: var(--ink-soft);
}

.workspace-heading > span:first-child {
  display: flex;
  gap: 8px;
  align-items: center;
  font-weight: 600;
}

.heading-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--brand);
}

.heading-caption {
  font-size: 9px;
  letter-spacing: 2px;
  color: var(--ink-faint);
}

.practice-hero {
  display: grid;
  grid-template-columns: 1fr 400px;
  position: relative;
  overflow: hidden;
  padding: 36px 42px;
  background: #142440;
  border: 1px solid #243957;
  border-radius: 16px;
  color: white;
}

.practice-hero::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(ellipse at 85% 10%, #315ee730, transparent 55%);
}

.hero-copy {
  position: relative;
  z-index: 1;
}

.hero-eyebrow {
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 2.6px;
  color: #94aace;
}

.hero-copy h1 {
  margin: 16px 0 14px;
  font-size: clamp(30px, 3.4vw, 44px);
  letter-spacing: 1px;
  line-height: 1.4;
  font-weight: 650;
}

.hero-period {
  color: #b3efcd;
}

.hero-copy p {
  margin: 0;
  font-size: 13px;
  color: #a9b8d1;
}

.hero-actions {
  margin-top: 25px;
  gap: 24px;
}

.hero-actions .btn {
  min-height: 42px;
  padding-inline: 20px;
}

.hero-text-link {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  font-size: 12px;
  color: #d0dbee;
  transition: color var(--transition);
}

.hero-text-link:hover {
  color: #b3efcd;
}

.hero-text-link .app-icon {
  width: 16px;
  height: 16px;
}

.hero-topics {
  display: flex;
  align-items: center;
  gap: 11px;
  margin-top: 22px;
  font-size: 10px;
  color: #7e95b8;
}

.hero-topics i {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: #62799d;
}

.algorithm-art {
  display: grid;
  justify-items: center;
  align-content: center;
  position: relative;
  padding: 4px 20px;
  background-image: radial-gradient(#7a99ce25 1px, transparent 1px);
  background-size: 20px 20px;
}

.art-label {
  display: flex;
  gap: 7px;
  align-items: center;
  font-size: 9px;
  letter-spacing: 2px;
  color: #9bafd0;
}

.art-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #b3efcd;
  box-shadow: 0 0 12px #b3efcd55;
}

.tree-illustration {
  width: 340px;
  height: 190px;
}

.art-code {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border: 1px solid #58709350;
  border-radius: 8px;
  background: #142440;
  color: #b7c9e5;
  font-size: 11px;
}

.code-symbol {
  color: #b3efcd;
}

.art-check .app-icon {
  width: 14px;
  height: 14px;
  color: #b3efcd;
}

.art-caption {
  margin-top: 12px;
  font-size: 10px;
  color: #6e88ae;
  letter-spacing: 2px;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.overview-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: white;
  position: relative;
  min-width: 0;
}

.stat-icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  background: var(--surface-tint);
  color: var(--brand);
  border-radius: 10px;
  flex-shrink: 0;
}

.stat-icon.teal {
  background: #eaf5f4;
  color: #258f86;
}

.stat-icon.purple {
  background: #f2effb;
  color: #9271cc;
}

.stat-icon.green {
  background: #eaf6ee;
  color: #458e60;
}

.overview-item > div {
  display: grid;
  gap: 1px;
}

.overview-item strong {
  font-size: 24px;
  letter-spacing: -.8px;
  line-height: 1.2;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.overview-item div span {
  font-size: 11px;
  color: var(--ink-faint);
}

.overview-item small {
  position: absolute;
  right: 14px;
  top: 13px;
  font-size: 7px;
  letter-spacing: .7px;
  color: #a4aec0;
}

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 268px;
  gap: 22px;
  align-items: start;
}

.problem-explorer {
  padding: 0;
  overflow: hidden;
}

.explorer-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 22px 24px 0;
}

.explorer-head h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.explorer-head h2 > .app-icon {
  color: var(--brand);
  width: 19px;
  height: 19px;
}

.count-badge {
  font-size: 11px;
  padding: 1px 7px;
  background: var(--bg-soft);
  color: var(--ink-faint);
  border-radius: 5px;
  margin-left: 2px;
}

.explorer-head p {
  font-size: 12px;
  color: var(--ink-faint);
  margin: 6px 0 0;
}

.search-bar {
  margin: 20px 24px 14px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #f9fafd;
  color: #8c98ad;
}

.search-bar:focus-within {
  border-color: var(--brand);
  box-shadow: 0 0 0 3px #315ee712;
}

.search-bar > .app-icon {
  width: 17px;
  height: 17px;
}

.search-bar input {
  width: 100%;
  min-width: 0;
  outline: 0;
  padding: 11px 0;
  border: 0;
  color: var(--ink);
  background: transparent;
  font-size: 12px;
}

.search-scope {
  flex-shrink: 0;
  font-size: 10px;
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 1px 5px;
}

.filter-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  padding: 0 24px 20px;
}

.filter-strip .tag-toggle {
  border-color: transparent;
  background: #f6f8fc;
  padding: 5px 10px;
  font-size: 11px;
}

.filter-strip .tag-toggle.active {
  background: var(--surface-tint);
  color: var(--brand);
  border-color: #315ee722;
}

.problem-row {
  display: grid;
  grid-template-columns: 100px minmax(0, 1fr) 180px 80px;
  gap: 14px;
  align-items: center;
  padding: 17px 24px;
  border-bottom: 1px solid #edf0f5;
}

.problem-head {
  background: #f9fafd;
  color: var(--ink-faint);
  font-size: 10px;
  padding-block: 12px;
  border-top: 1px solid var(--line);
}

.problem-item {
  transition: background var(--transition);
}

.problem-item:hover {
  background: #f8faff;
}

.problem-item:last-child {
  border-bottom: 0;
}

.problem-id {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: #8c99ac;
}

.solve-status {
  display: flex;
  color: #bec7d6;
}

.solve-status .app-icon {
  width: 15px;
  height: 15px;
}

.solve-status.solved {
  color: var(--success);
  background: #e7f6ef;
  border-radius: 50%;
  padding: 2px;
  margin-left: -2px;
  margin-right: -2px;
}

.problem-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.problem-main strong {
  font-size: 13px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.problem-item:hover .problem-main strong {
  color: var(--brand);
}

.problem-main > span {
  font-size: 10px;
  color: #95a0b1;
}

.problem-main b {
  font-weight: 400;
  margin: 0 5px;
  color: #b8c2d1;
}

.problem-tags {
  display: flex;
  gap: 5px;
  flex-wrap: wrap;
  min-width: 0;
}

.problem-tags .mini-tag {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 10px;
  padding: 3px 7px;
}

.problem-rate {
  display: grid;
  gap: 6px;
  justify-items: end;
}

.problem-rate strong {
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.problem-rate small {
  font-size: 10px;
  color: var(--ink-faint);
  margin-left: 1px;
}

.rate-track {
  width: 58px;
  height: 3px;
  background: #edf0f6;
  border-radius: 3px;
  overflow: hidden;
}

.rate-track i {
  display: block;
  height: 100%;
  background: #84b7a3;
}

.explorer-footer {
  padding: 16px 24px;
  border-top: 1px solid var(--line);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.explorer-footer > span {
  color: var(--ink-faint);
  font-size: 10px;
}

.workspace-sidebar {
  display: grid;
  gap: 16px;
}

.assistant-card {
  border-radius: 14px;
  border: 1px solid #dfe7fc;
  padding: 24px;
  background: linear-gradient(145deg, #eef3ff, #f9fbff);
}

.assistant-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 11px;
  color: var(--brand);
  background: white;
  border: 1px solid #e0e7fb;
  margin-bottom: 20px;
}

.assistant-icon .app-icon {
  width: 23px;
  height: 23px;
}

.sidebar-kicker {
  font-size: 7px;
  color: #899ac1;
  letter-spacing: 1.3px;
}

.assistant-card h3 {
  margin: 10px 0;
  font-size: 22px;
  line-height: 1.5;
  font-weight: 650;
  letter-spacing: .3px;
}

.assistant-card p, .sidebar-card p {
  font-size: 12px;
  line-height: 1.9;
  color: var(--ink-faint);
  margin: 0 0 20px;
}

.assistant-note {
  display: block;
  text-align: center;
  font-size: 10px;
  color: #9aa9c4;
  margin-top: 10px;
}

.sidebar-card {
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 22px;
  background: white;
}

.sidebar-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.sidebar-title .app-icon {
  width: 18px;
  height: 18px;
  color: #ba8d3b;
}

.sidebar-title h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 650;
}

.sidebar-card p {
  margin-bottom: 14px;
}

.sidebar-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--brand);
  font-size: 11px;
  font-weight: 600;
}

.sidebar-link .app-icon {
  width: 16px;
  height: 16px;
}

.practice-note {
  padding: 8px 12px;
}

.note-number {
  font-size: 8px;
  letter-spacing: 1.7px;
  color: #97a2b5;
}

.practice-note p {
  color: #8793a7;
  font-size: 11px;
  line-height: 2;
  margin: 10px 0;
}

.practice-note > span:last-child {
  font-family: Georgia, serif;
  font-style: italic;
  font-size: 13px;
  color: #9aa5b8;
}

.table-skeleton {
  padding: 0 24px;
}

.skeleton-row {
  display: grid;
  grid-template-columns: 60px 1fr 70px;
  gap: 28px;
  padding: 24px 0;
  border-top: 1px solid var(--line);
}

.skeleton-row span {
  height: 12px;
  border-radius: 4px;
  background: #edf1f8;
  animation: pulse 1.4s ease-in-out infinite;
}

.skeleton-row span:nth-child(2) {
  width: 70%;
}

.problem-explorer .empty-state {
  border: 0;
  border-radius: 0;
  min-height: 280px;
}

.empty-state > .app-icon {
  width: 30px;
  height: 30px;
  color: #9cacc9;
}

.tags-error {
  padding: 0 24px;
  font-size: 11px;
  color: var(--danger);
}

.tags-error button {
  color: var(--brand);
  text-decoration: underline;
}

@keyframes pulse {
  50% {
    opacity: .45;
  }
}

@media (max-width: 1150px) {
  .practice-hero {
    grid-template-columns: 1fr 330px;
    padding: 30px;
  }
  .workspace-grid {
    grid-template-columns: minmax(0, 1fr) 236px;
    gap: 16px;
  }
  .problem-row {
    grid-template-columns: 78px minmax(0, 1fr) 110px 65px;
    gap: 12px;
  }
  .overview-item {
    padding: 18px 14px;
  }
  .overview-item small {
    display: none;
  }
  .assistant-card {
    padding: 20px;
  }
}

@media (max-width: 940px) {
  .workspace-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .workspace-sidebar {
    grid-template-columns: 1fr 1fr;
  }
  .assistant-card h3 br {
    display: none;
  }
  .practice-note {
    display: none;
  }
  .assistant-card .assistant-icon {
    margin-bottom: 10px;
  }
  .assistant-card .btn {
    width: fit-content;
  }
  .assistant-note {
    text-align: left;
  }
  .algorithm-art {
    padding: 0;
  }
  .hero-copy p {
    max-width: 340px;
  }
  .overview-item {
    gap: 10px;
  }
  .stat-icon {
    width: 32px;
    height: 32px;
  }
  .stat-icon .app-icon {
    width: 17px;
    height: 17px;
  }
}

@media (max-width: 680px) {
  .practice-hero {
    grid-template-columns: 1fr;
    padding: 28px 24px;
  }
  .algorithm-art {
    display: none;
  }
  .hero-copy h1 {
    font-size: 32px;
  }
  .hero-copy p {
    max-width: none;
  }
  .overview-grid {
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .overview-item {
    padding: 16px;
  }
  .workspace-sidebar {
    grid-template-columns: 1fr;
  }
  .sidebar-card, .practice-note {
    display: none;
  }
  .assistant-card h3 {
    font-size: 20px;
  }
  .workspace-heading .heading-caption {
    display: none;
  }
  .problem-row {
    grid-template-columns: 60px minmax(0, 1fr) 55px;
    padding: 16px;
    gap: 12px;
  }
  .problem-head > :nth-child(3), .problem-tags {
    display: none;
  }
  .problem-id {
    gap: 6px;
  }
  .problem-main > span {
    font-size: 9px;
  }
  .problem-main b {
    margin: 0 2px;
  }
  .explorer-head {
    padding: 20px 16px 0;
  }
  .search-bar {
    margin-inline: 16px;
  }
  .filter-strip {
    padding-inline: 16px;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: thin;
  }
  .filter-strip .tag-toggle {
    flex-shrink: 0;
    white-space: nowrap;
  }
  .search-bar input {
    font-size: 16px;
  }
  .explorer-footer {
    flex-direction: column;
    align-items: flex-start;
    padding: 16px;
  }
  .table-skeleton {
    padding-inline: 16px;
  }
}
</style>
