<template>
  <div class="dashboard-stats">
    <div class="stat-strip panel">
      <div class="stat-item">
        <span class="stat-label">{{ t('dashboard.stats.totalBooks') }}</span>
        <span class="stat-value">{{ totalBooks }}</span>
      </div>

      <div class="stat-item">
        <span class="stat-label">{{ t('dashboard.stats.inProgress') }}</span>
        <span class="stat-value">{{ inProgress }}</span>
      </div>

      <div class="stat-item">
        <span class="stat-label">{{ t('dashboard.stats.avgStar') }}</span>
        <span class="stat-value">{{ starAvgDisplay }}</span>
      </div>

      <div class="stat-item">
        <span class="stat-label">{{ t('dashboard.stats.totalChars') }}</span>
        <span class="stat-value">{{ totalCharsDisplay }}</span>
      </div>

      <div class="stat-item">
        <span class="stat-label">{{ t('dashboard.stats.currentStreak') }}</span>
        <span class="stat-value">{{ t('dashboard.stats.currentStreakValue', { days: currentStreak }) }}</span>
      </div>
    </div>

    <div class="rating-distribution panel">
      <span class="stat-label">{{ t('dashboard.stats.ratingDistribution') }}</span>
      <div class="star-distribution" role="img" :aria-label="starDistributionAriaLabel">
        <div v-for="star in [5, 4, 3, 2, 1]" :key="star" class="star-bar-row">
          <span class="star-bar-label">{{ star }}★</span>
          <span class="star-bar-track">
            <span
              class="star-bar-fill"
              :style="{ width: `${starBarWidth(star)}%` }"
            ></span>
          </span>
          <span class="star-bar-count">{{ starCount(star) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from '@/i18n';
import type { StarDistribution } from '@/features/dashboard/composables/useDashboardData';

const props = defineProps<{
  totalBooks: number;
  inProgress: number;
  starAvg: number | null;
  starDistribution: StarDistribution;
  totalChars: number | null;
  currentStreak: number;
}>();

const { t } = useI18n();

const starAvgDisplay = computed(() => (props.starAvg === null ? '—' : props.starAvg.toFixed(1)));

const totalCharsDisplay = computed(() =>
  props.totalChars === null ? '—' : props.totalChars.toLocaleString()
);

function starCount(star: number): number {
  switch (star) {
    case 1: return props.starDistribution[1];
    case 2: return props.starDistribution[2];
    case 3: return props.starDistribution[3];
    case 4: return props.starDistribution[4];
    case 5: return props.starDistribution[5];
    default: return 0;
  }
}

const maxStarCount = computed(() => Math.max(1, starCount(1), starCount(2), starCount(3), starCount(4), starCount(5)));

function starBarWidth(star: number): number {
  const count = starCount(star);
  if (count === 0) {
    return 0;
  }
  return Math.max(4, Math.round((count / maxStarCount.value) * 100));
}

const starDistributionAriaLabel = computed(() =>
  [5, 4, 3, 2, 1]
    .map((star) => t('dashboard.stats.starBar', { star, count: starCount(star) }))
    .join('; ')
);
</script>

<style scoped>
.dashboard-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

/* The five headline numbers sit on one line; each item grows to share the row
   but never shrinks below a legible width, wrapping only when the row cannot
   hold them. */
.stat-strip {
  align-items: stretch;
  display: flex;
  flex: 1 1 360px;
  flex-wrap: wrap;
  gap: 8px 24px;
  padding: 14px 18px;
}

.stat-item {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: 4px;
  min-width: 96px;
}

.stat-label {
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.stat-value {
  color: var(--text);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

/* Rating distribution is its own block now, no longer nested in a stat. */
.rating-distribution {
  display: flex;
  flex: 0 1 260px;
  flex-direction: column;
  gap: 6px;
  padding: 14px 18px;
}

.star-distribution {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.star-bar-row {
  align-items: center;
  display: flex;
  gap: 6px;
}

.star-bar-label {
  color: var(--muted);
  flex: 0 0 auto;
  font-size: 11px;
  width: 24px;
}

.star-bar-track {
  background: var(--bg);
  border-radius: 3px;
  flex: 1;
  height: 6px;
  min-width: 0;
  overflow: hidden;
}

.star-bar-fill {
  background: var(--accent);
  border-radius: 3px;
  display: block;
  height: 100%;
  transition: width 0.15s ease;
}

.star-bar-count {
  color: var(--muted);
  flex: 0 0 auto;
  font-size: 11px;
  text-align: right;
  width: 18px;
}
</style>
