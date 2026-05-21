<template>
  <div class="shell">
    <div class="shell-noise"></div>
    <div class="shell-glow"></div>
    <Navbar />
    <main class="shell-main">
      <router-view v-slot="{ Component }">
        <transition name="fade-slide" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
    <footer class="shell-footer">
      <div class="footer-card">
        <div>
          <div class="footer-brand">GoJo</div>
          <div class="footer-caption">A polished Online Judge demo built around your Go backend.</div>
        </div>
        <div class="footer-caption">Vue Frontend · Go API · Redis Queue · Docker Sandbox</div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import Navbar from './components/Navbar.vue'
</script>
