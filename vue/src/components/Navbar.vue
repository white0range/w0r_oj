<template>
  <header class="navbar">
    <div class="navbar-shell">
      <router-link to="/" class="brand">
        <span class="brand-mark">GJ</span>
        <div class="brand-copy">
          <strong>GoJo</strong>
          <span>Online Judge Studio</span>
        </div>
      </router-link>

      <nav class="nav-links desktop-only">
        <router-link to="/" class="nav-link">题库</router-link>
        <router-link to="/leaderboard" class="nav-link">排行榜</router-link>
        <router-link v-if="store.isLoggedIn" to="/my-submissions" class="nav-link">提交记录</router-link>
        <router-link v-if="store.isAdmin" to="/admin/problems" class="nav-link nav-admin">管理后台</router-link>
      </nav>

      <div class="nav-actions desktop-only">
        <template v-if="store.isLoggedIn">
          <router-link to="/profile" class="profile-chip" :class="{ admin: store.isAdmin }">
            <span class="profile-avatar">{{ initials }}</span>
            <span>{{ store.username }}</span>
            <span v-if="store.isAdmin" class="badge badge-admin">Admin</span>
          </router-link>
          <button class="btn btn-ghost btn-sm" @click="logout">退出</button>
        </template>
        <template v-else>
          <router-link to="/login" class="btn btn-ghost btn-sm">登录</router-link>
          <router-link to="/register" class="btn btn-primary btn-sm">注册</router-link>
        </template>
      </div>

      <button class="mobile-toggle" @click="menuOpen = !menuOpen">
        <span></span>
        <span></span>
      </button>
    </div>

    <transition name="fade-slide">
      <div v-if="menuOpen" class="mobile-panel">
        <router-link to="/" class="mobile-link" @click="closeMenu">题库</router-link>
        <router-link to="/leaderboard" class="mobile-link" @click="closeMenu">排行榜</router-link>
        <router-link v-if="store.isLoggedIn" to="/my-submissions" class="mobile-link" @click="closeMenu">提交记录</router-link>
        <router-link v-if="store.isLoggedIn" to="/profile" class="mobile-link" @click="closeMenu">个人中心</router-link>
        <router-link v-if="store.isAdmin" to="/admin/problems" class="mobile-link" @click="closeMenu">管理后台</router-link>
        <div class="mobile-actions">
          <template v-if="store.isLoggedIn">
            <button class="btn btn-ghost btn-block" @click="logout">退出当前账号</button>
          </template>
          <template v-else>
            <router-link to="/login" class="btn btn-ghost btn-block" @click="closeMenu">登录</router-link>
            <router-link to="/register" class="btn btn-primary btn-block" @click="closeMenu">注册</router-link>
          </template>
        </div>
      </div>
    </transition>
  </header>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { store } from '../store'

const menuOpen = ref(false)
const route = useRoute()
const router = useRouter()

const initials = computed(() => (store.username || 'G').slice(0, 1).toUpperCase())

watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
  },
)

function closeMenu() {
  menuOpen.value = false
}

function logout() {
  store.logout()
  closeMenu()
  router.push('/')
}
</script>

<style scoped>
.navbar {
  position: sticky;
  top: 0;
  z-index: 30;
  padding: 18px 16px 0;
}

.navbar-shell,
.mobile-panel {
  width: min(100%, var(--container));
  margin: 0 auto;
  border: 1px solid rgba(20, 33, 61, 0.1);
  border-radius: 999px;
  background: rgba(255, 252, 247, 0.78);
  backdrop-filter: blur(18px);
  box-shadow: var(--shadow-sm);
}

.navbar-shell {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--ink);
}

.brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--brand), var(--brand-deep));
  color: #fff7f2;
  font-size: 14px;
  font-weight: 800;
  box-shadow: 0 14px 24px rgba(143, 61, 34, 0.22);
}

.brand-copy {
  display: grid;
}

.brand-copy strong {
  font-size: 17px;
  letter-spacing: -0.03em;
}

.brand-copy span {
  font-size: 12px;
  color: var(--muted);
}

.nav-links,
.nav-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.nav-link {
  padding: 10px 14px;
  border-radius: 999px;
  color: var(--ink-soft);
  font-weight: 700;
  transition: all var(--transition);
}

.nav-link.router-link-active,
.nav-link:hover {
  color: var(--ink);
  background: rgba(20, 33, 61, 0.06);
}

.nav-admin {
  color: var(--brand-deep);
}

.profile-chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px 7px 8px;
  border-radius: 999px;
  background: rgba(20, 33, 61, 0.06);
  color: var(--ink);
  font-weight: 700;
}

.profile-chip.admin {
  background: rgba(203, 109, 67, 0.12);
}

.profile-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--accent), #125b56);
  color: #f0fffc;
  font-size: 13px;
  font-weight: 800;
}

.mobile-toggle {
  display: none;
  flex-direction: column;
  gap: 5px;
  padding: 8px;
  cursor: pointer;
}

.mobile-toggle span {
  width: 22px;
  height: 2px;
  border-radius: 999px;
  background: var(--ink);
}

.mobile-panel {
  margin-top: 12px;
  border-radius: 28px;
  padding: 18px;
  display: grid;
  gap: 10px;
}

.mobile-link {
  padding: 12px 14px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.55);
  font-weight: 700;
  color: var(--ink);
}

.mobile-actions {
  display: grid;
  gap: 10px;
  padding-top: 8px;
}

@media (max-width: 860px) {
  .desktop-only {
    display: none;
  }

  .mobile-toggle {
    display: inline-flex;
  }

  .navbar-shell,
  .mobile-panel {
    width: 100%;
  }
}
</style>
