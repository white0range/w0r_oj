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
          <div class="footer-brand">Gojo OJ</div>
          <div class="footer-caption">
            Professional online judge for algorithm practice, async judging, progress tracking, and AI-assisted learning workflows.
          </div>
        </div>
        <div class="footer-caption footer-tech">
          Go API · Redis Queue · Docker Sandbox · Elasticsearch · Python Agent · Vue Console
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import Navbar from './components/Navbar.vue'
</script>
