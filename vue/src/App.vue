<template>
  <div class="shell">
    <a class="skip-link" href="#main-content" @click.prevent="skipToContent">跳转到主要内容</a>
    <Navbar />
    <main id="main-content" class="shell-main" tabindex="-1">
      <router-view v-slot="{ Component }">
        <transition name="fade-slide" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
    <footer class="shell-footer">
      <div class="footer-card">
        <div>
          <div class="footer-brand">Gojo OJ</div>
          <div class="footer-caption">
            专注算法，持续进步。
          </div>
        </div>
        <nav class="footer-links" aria-label="页脚导航">
          <router-link to="/">题库</router-link>
          <router-link to="/leaderboard">排行榜</router-link>
          <router-link to="/chat">AI 学习助手</router-link>
        </nav>
      </div>
      <div class="footer-filing">
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">
          鲁ICP备2026058391号-1
        </a>
      </div>
    </footer>
  </div>
</template>

<script setup>
import Navbar from './components/Navbar.vue'

function skipToContent() {
  document.getElementById('main-content')?.focus()
}
</script>

