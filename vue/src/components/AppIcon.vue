<template>
  <svg class="app-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path v-for="(path, index) in paths[name] || paths.code" :key="index" :d="path" />
  </svg>
</template>

<script setup>
defineProps({ name: { type: String, default: 'code' } })
const paths = {
  code: ['m8 7-5 5 5 5', 'm16 7 5 5-5 5', 'm14 4-4 16'],
  book: ['M4 4h6a2 2 0 0 1 2 2v14a3 3 0 0 0-3-2H4z', 'M20 4h-6a2 2 0 0 0-2 2v14a3 3 0 0 1 3-2h5z'],
  trophy: ['M8 3h8v6a4 4 0 0 1-8 0z', 'M8 5H4v2a4 4 0 0 0 4 4', 'M16 5h4v2a4 4 0 0 1-4 4', 'M12 13v5', 'M8 21h8l-1-3H9z'],
  sparkles: ['m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5z', 'M20 2v4', 'M18 4h4'],
  history: ['M3 11a9 9 0 1 1 2 7', 'M3 4v7h7', 'M12 7v5l3 2'],
  user: ['M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0', 'M4 21v-2a8 8 0 0 1 16 0v2'],
  grid: ['M3 3h7v7H3z', 'M14 3h7v7h-7z', 'M3 14h7v7H3z', 'M14 14h7v7h-7z'],
  arrow: ['M4 12h16', 'm14 6 6 6-6 6'],
  chevron: ['m9 5 7 7-7 7'],
  search: ['M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0', 'm15 15 6 6'],
  check: ['m5 12 4 4L19 6'],
  circle: ['M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0'],
  tag: ['M3 3h7l11 11-7 7L3 10z', 'M7 7h.01'],
  clock: ['M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0', 'M12 7v5l3 2'],
  menu: ['M4 6h16', 'M4 12h16', 'M4 18h16'],
  close: ['m6 6 12 12', 'M18 6 6 18'],
  logout: ['M9 4H4v16h5', 'M9 12h12', 'm17 8 4 4-4 4'],
  refresh: ['M20 11a8 8 0 0 0-14-5L3 9', 'M3 3v6h6', 'M4 13a8 8 0 0 0 14 5l3-3', 'M21 21v-6h-6'],
  lock: ['M5 10h14v11H5z', 'M8 10V7a4 4 0 0 1 8 0v3', 'M12 14v3'],
}
</script>

<style scoped>
.app-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  vertical-align: middle;
}
</style>
