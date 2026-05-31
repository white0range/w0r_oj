<template>
  <div class="shell">
    <div class="shell-layer"></div>
    <div class="shell-grid"></div>
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
          <div class="footer-brand">Gojo OJ</div>
          <div class="footer-caption">
            A learning-oriented online judge with async judging, AI analysis, study planning, RAG, and memory.
          </div>
        </div>
        <div class="footer-caption">Vue Frontend · Go API · Redis Queue · Docker Sandbox · Python Agent</div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import Navbar from './components/Navbar.vue'
</script>

<style scoped>
.footer-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 22px 26px;
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  background: rgba(255, 253, 249, 0.78);
  backdrop-filter: blur(18px);
  box-shadow: var(--shadow-sm);
  color: var(--ink-soft);
}

.footer-brand {
  font-weight: 700;
  color: var(--ink);
}

.footer-caption {
  font-size: 14px;
  color: var(--ink-faint);
}

@media (max-width: 640px) {
  .footer-card {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
