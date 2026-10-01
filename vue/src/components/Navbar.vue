<template>
  <header class="navbar" @keydown.esc="menuOpen = false">
    <div class="navbar-shell">
      <router-link to="/" class="brand" aria-label="Gojo OJ 首页">
        <span class="brand-mark"><AppIcon name="code" /></span>
        <strong>Gojo<span>OJ</span><small>ONLINE JUDGE</small></strong>
      </router-link>
      <nav class="nav-links desktop-only" aria-label="主导航">
        <router-link v-for="item in navigation" :key="item.to" :to="item.to" class="nav-link" :class="{ 'nav-active': isActive(item.to) }">
          <AppIcon :name="item.icon" />{{ item.label }}
        </router-link>
      </nav>
      <div class="nav-actions desktop-only">
        <template v-if="store.isLoggedIn">
          <router-link to="/profile" class="profile-chip"><span class="profile-avatar">{{ initials }}</span><strong>{{ store.username }}</strong></router-link>
          <button class="icon-button" aria-label="退出登录" title="退出登录" @click="logout"><AppIcon name="logout" /></button>
        </template>
        <template v-else>
          <router-link to="/login" class="btn btn-ghost btn-sm">登录</router-link>
          <router-link to="/register" class="btn btn-primary btn-sm">开始练习 <AppIcon name="arrow" /></router-link>
        </template>
      </div>
      <button class="mobile-toggle icon-button" :aria-expanded="menuOpen" aria-controls="mobile-navigation" :aria-label="menuOpen ? '关闭导航' : '打开导航'" @click="menuOpen = !menuOpen"><AppIcon :name="menuOpen ? 'close' : 'menu'" /></button>
    </div>
    <transition name="fade-slide">
      <nav v-if="menuOpen" id="mobile-navigation" class="mobile-panel" aria-label="手机导航">
        <router-link v-for="item in navigation" :key="item.to" :to="item.to" class="mobile-link" :class="{ 'nav-active': isActive(item.to) }"><AppIcon :name="item.icon" />{{ item.label }}</router-link>
        <router-link v-if="store.isLoggedIn" to="/profile" class="mobile-link"><AppIcon name="user" />个人中心</router-link>
        <div class="mobile-actions">
          <button v-if="store.isLoggedIn" class="btn btn-outline btn-block" @click="logout">退出登录</button>
          <template v-else><router-link to="/login" class="btn btn-outline">登录</router-link><router-link to="/register" class="btn btn-primary">开始练习</router-link></template>
        </div>
      </nav>
    </transition>
  </header>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { logoutUser } from '../api'
import { store } from '../store'
import AppIcon from './AppIcon.vue'

const menuOpen = ref(false)
const route = useRoute()
const router = useRouter()
const initials = computed(() => (store.username || 'G').slice(0, 1).toUpperCase())
const navigation = computed(() => [
  { to: '/', label: '题库', icon: 'book' },
  { to: '/leaderboard', label: '排行榜', icon: 'trophy' },
  { to: '/chat', label: 'AI 学习助手', icon: 'sparkles' },
  ...(store.isLoggedIn ? [{ to: '/my-submissions', label: '提交记录', icon: 'history' }] : []),
  ...(store.isAdmin ? [{ to: '/admin/users', label: '管理后台', icon: 'grid' }] : []),
])
function isActive(path) {
  if (path === '/') return route.path === '/' || route.path.startsWith('/problems/')
  if (path.startsWith('/admin')) return route.path.startsWith('/admin')
  return route.path === path
}
watch(() => route.fullPath, () => { menuOpen.value = false })
async function logout() {
  try { await logoutUser() } finally { menuOpen.value = false; router.push('/') }
}
</script>
<style scoped>
.navbar {
  position: sticky;
  top: 0;
  z-index: 40;
  border-bottom: 1px solid var(--line);
  background: rgba(255,255,255,.96);
  backdrop-filter: blur(16px);
}

.navbar-shell {
  display: flex;
  align-items: center;
  gap: 32px;
  width: min(calc(100% - 64px), var(--container));
  height: 76px;
  margin: auto;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.brand-mark {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 11px;
  color: white;
  background: var(--brand);
  box-shadow: 0 3px 8px #275be52b;
}

.brand-mark .app-icon {
  width: 25px;
  height: 25px;
}

.brand strong {
  font-size: 22px;
  line-height: 1.1;
  letter-spacing: -.7px;
}

.brand strong > span {
  color: var(--brand);
  margin-left: 4px;
}

.brand small {
  display: block;
  font-size: 8px;
  font-weight: 600;
  letter-spacing: 2.5px;
  color: var(--ink-faint);
  margin-top: 5px;
}

.nav-links {
  display: flex;
  align-items: stretch;
  gap: 5px;
  height: 100%;
  margin-left: 20px;
}

.nav-link {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  position: relative;
  padding: 0 14px;
  color: var(--ink-faint);
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
  transition: color var(--transition);
}

.nav-link .app-icon {
  width: 17px;
  height: 17px;
}

.nav-link:hover, .nav-active {
  color: var(--brand);
}

.nav-link.nav-active::after {
  content: '';
  position: absolute;
  height: 3px;
  background: var(--brand);
  bottom: -1px;
  left: 14px;
  right: 14px;
  border-radius: 3px 3px 0 0;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}

.profile-chip {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 13px;
}

.profile-chip strong {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-avatar {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  color: var(--brand);
  background: #eef3ff;
  font-weight: 700;
}

.mobile-toggle, .mobile-panel {
  display: none;
}

@media (max-width: 1100px) {
  .navbar-shell {
    gap: 16px;
  }
  .nav-links {
    margin-left: 0;
  }
  .nav-link {
    padding-inline: 10px;
  }
  .nav-link .app-icon {
    display: none;
  }
}

@media (max-width: 860px) {
  .navbar-shell {
    width: calc(100% - 32px);
    height: 64px;
  }
  .desktop-only {
    display: none;
  }
  .mobile-toggle {
    display: inline-flex;
    margin-left: auto;
  }
  .mobile-panel {
    display: grid;
    gap: 4px;
    padding: 12px 16px 18px;
    border-top: 1px solid var(--line);
    box-shadow: var(--shadow-md);
    max-height: calc(100dvh - 64px);
    overflow-y: auto;
  }
  .mobile-link {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 12px;
    border-radius: 8px;
    font-weight: 600;
  }
  .mobile-link.nav-active {
    background: var(--surface-tint);
  }
  .mobile-actions {
    display: flex;
    gap: 10px;
    padding: 12px 0 0;
    border-top: 1px solid var(--line);
    margin-top: 8px;
  }
  .mobile-actions .btn {
    flex: 1;
  }
}
</style>
