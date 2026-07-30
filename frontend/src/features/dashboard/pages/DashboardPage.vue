<template>
  <section class="dashboard-page">
    <header class="dashboard-header">
      <h2>{{ t('dashboard.title') }}</h2>
      <button type="button" class="button" :disabled="loading" @click="fetchDashboardData">
        {{ t('dashboard.refresh') }}
      </button>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-else-if="loading" class="loading">{{ t('dashboard.loading') }}</p>

    <div v-else class="dashboard-grid">
      <ReadingHeatmap class="dashboard-cell dashboard-cell-heatmap" :data="heatmapData" />
      <StatsCards
        class="dashboard-cell dashboard-cell-stats"
        :total-books="totalBooks"
        :added-this-month="addedThisMonth"
        :star-avg="starAvg"
        :star-distribution="starDistribution"
        :total-chars="totalChars"
        :current-streak="currentStreak"
      />
      <TagCloud class="dashboard-cell dashboard-cell-tags" :tag-counts="tagCounts" />
      <RandomBook class="dashboard-cell dashboard-cell-random" :books="books" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import StatsCards from '@/features/dashboard/components/StatsCards.vue';
import TagCloud from '@/features/dashboard/components/TagCloud.vue';
import RandomBook from '@/features/dashboard/components/RandomBook.vue';
import ReadingHeatmap from '@/features/dashboard/components/ReadingHeatmap.vue';
import { useDashboardData } from '@/features/dashboard/composables/useDashboardData';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const {
  books,
  loading,
  error,
  totalBooks,
  addedThisMonth,
  starAvg,
  starDistribution,
  totalChars,
  tagCounts,
  heatmapData,
  currentStreak,
  fetchDashboardData
} = useDashboardData();

useDocumentTitle(() => [t('dashboard.title'), 'PlainShelf']);

onMounted(() => {
  void fetchDashboardData();
});
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dashboard-header {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.dashboard-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.dashboard-cell-stats,
.dashboard-cell-heatmap {
  grid-column: 1 / -1;
}

.dashboard-cell-heatmap {
  box-sizing: border-box;
  width: 100%;
}

@media (max-width: 900px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }

  .dashboard-cell-stats,
  .dashboard-cell-tags,
  .dashboard-cell-random,
  .dashboard-cell-heatmap {
    grid-column: 1 / -1;
  }
}
</style>
