<template>
  <div class="reader-layout">
    <RouterView />
  </div>
</template>

<script setup lang="ts"></script>

<style scoped>
.reader-layout {
  height: 100vh;
  width: 100vw;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  box-sizing: border-box;
  background:
    radial-gradient(circle at top, rgba(255, 250, 240, 0.92), rgba(245, 241, 232, 0.96) 42%),
    linear-gradient(180deg, #f7f2e7 0%, #efe7d7 100%);
}
</style>
