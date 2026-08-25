<template>
  <!-- Mirrors DashboardPage's grid regions so swapping in the real content on
       load does not shift the layout. Decorative only: hidden from assistive
       tech, which hears the polite loading status instead. -->
  <div class="dashboard-skeleton dashboard-grid" aria-hidden="true">
    <div class="skeleton-cell skeleton-recent dashboard-cell-recent"></div>
    <div class="skeleton-cell skeleton-stats dashboard-cell-stats"></div>
    <div class="skeleton-cell skeleton-tags"></div>
    <div class="skeleton-cell skeleton-random"></div>
    <div class="skeleton-cell skeleton-heatmap dashboard-cell-heatmap"></div>
  </div>
</template>

<script setup lang="ts"></script>

<style scoped>
.dashboard-skeleton {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.dashboard-cell-recent,
.dashboard-cell-stats,
.dashboard-cell-heatmap {
  grid-column: 1 / -1;
}

.skeleton-cell {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  position: relative;
  overflow: hidden;
}

.skeleton-cell::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(
    90deg,
    transparent 0%,
    color-mix(in srgb, var(--text) 8%, transparent) 50%,
    transparent 100%
  );
  transform: translateX(-100%);
  animation: skeleton-shimmer 1.4s ease-in-out infinite;
}

.skeleton-recent {
  min-height: 148px;
}

.skeleton-stats {
  min-height: 116px;
}

.skeleton-tags,
.skeleton-random {
  min-height: 176px;
}

.skeleton-heatmap {
  min-height: 168px;
}

@keyframes skeleton-shimmer {
  100% {
    transform: translateX(100%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .skeleton-cell::after {
    animation: none;
  }
}

@media (max-width: 900px) {
  .dashboard-skeleton {
    grid-template-columns: 1fr;
  }

  .skeleton-tags,
  .skeleton-random {
    grid-column: 1 / -1;
  }
}
</style>
