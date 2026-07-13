<template>
  <section class="dashboard-page">
    <header class="dashboard-header">
      <h2>{{ t('dashboard.title') }}</h2>
      <button type="button" class="button" :disabled="loading" @click="fetchDashboardData">
        {{ t('common.retry') }}
      </button>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-else-if="loading" class="loading">{{ t('dashboard.loading') }}</p>

    <div v-else class="dashboard-grid">
      <StatsCards
        class="dashboard-cell dashboard-cell-stats"
        :total-books="totalBooks"
        :added-this-month="addedThisMonth"
        :star-avg="starAvg"
        :star-distribution="starDistribution"
        :total-chars="totalChars"
      />
      <TagCloud class="dashboard-cell dashboard-cell-tags" :tag-counts="tagCounts" />
      <RandomBook class="dashboard-cell dashboard-cell-random" :books="books" />
      <ReadingHeatmap class="dashboard-cell dashboard-cell-heatmap" :data="readingHeatmapData" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import StatsCards from '../components/StatsCards.vue';
import TagCloud from '../components/TagCloud.vue';
import RandomBook from '../components/RandomBook.vue';
import ReadingHeatmap from '../components/ReadingHeatmap.vue';
import { useDashboardData } from '../composables/useDashboardData';
import { useDocumentTitle } from '../../../composables/useDocumentTitle';
import { useI18n } from '../../../i18n';

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
  fetchDashboardData
} = useDashboardData();

// TODO(next phase): wire up real reading-history data (per-day seconds read)
// once that store/provider exists; an empty map renders the heatmap's empty
// state, which is the intended skeleton behavior for this task.
const readingHeatmapData: Record<string, number> = {};

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
