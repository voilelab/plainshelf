<template>
  <section class="dashboard-page">
    <header class="dashboard-header">
      <h2>{{ t('dashboard.title') }}</h2>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <template v-else-if="loading">
      <!-- A cold start keeps loading true while it retries the 503; surface that
           status so the wait reads as "still starting" rather than a blank load. -->
      <p v-if="shelfInitializing" class="loading-status" role="status">
        {{ t('dashboard.shelfInitializing') }}
      </p>
      <DashboardSkeleton />
    </template>

    <!-- Only reached once loading is false with no error: a genuine empty shelf,
         never a shelf that is still initializing (that path keeps loading true). -->
    <EmptyShelf v-else-if="books.length === 0" />

    <div v-else class="dashboard-grid">
      <RecentReading class="dashboard-cell dashboard-cell-recent" :items="recentReading" />
      <StatsCards
        class="dashboard-cell dashboard-cell-stats"
        :total-books="totalBooks"
        :in-progress="inProgress"
        :star-avg="starAvg"
        :star-distribution="starDistribution"
        :total-chars="totalChars"
        :current-streak="currentStreak"
      />
      <TagCloud class="dashboard-cell dashboard-cell-tags" :tag-counts="tagCounts" />
      <RandomBook class="dashboard-cell dashboard-cell-random" :books="books" />
      <ReadingHeatmap class="dashboard-cell dashboard-cell-heatmap" :data="heatmapData" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue';
import StatsCards from '@/features/dashboard/components/StatsCards.vue';
import TagCloud from '@/features/dashboard/components/TagCloud.vue';
import RandomBook from '@/features/dashboard/components/RandomBook.vue';
import RecentReading from '@/features/dashboard/components/RecentReading.vue';
import ReadingHeatmap from '@/features/dashboard/components/ReadingHeatmap.vue';
import DashboardSkeleton from '@/features/dashboard/components/DashboardSkeleton.vue';
import EmptyShelf from '@/features/dashboard/components/EmptyShelf.vue';
import { useDashboardData } from '@/features/dashboard/composables/useDashboardData';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const {
  books,
  loading,
  error,
  recentReading,
  shelfInitializing,
  totalBooks,
  inProgress,
  starAvg,
  starDistribution,
  totalChars,
  tagCounts,
  heatmapData,
  currentStreak,
  fetchDashboardData
} = useDashboardData();

useDocumentTitle(() => [t('dashboard.title'), 'PlainShelf']);

// Refetch when the window regains focus so a book imported or read in another
// window shows up on return. The Refresh button is gone; entering the route
// (a fresh mount) and this focus handler are what keep the page current. Skip
// while a load or a 503 retry is already in flight so focus does not stack
// requests onto the cold-start retry loop.
function refetchOnFocus(): void {
  if (loading.value) {
    return;
  }
  void fetchDashboardData();
}

onMounted(() => {
  void fetchDashboardData();
  window.addEventListener('focus', refetchOnFocus);
});

onBeforeUnmount(() => {
  window.removeEventListener('focus', refetchOnFocus);
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
}

.loading-status {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}

.dashboard-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.dashboard-cell-recent,
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

  .dashboard-cell-recent,
  .dashboard-cell-stats,
  .dashboard-cell-tags,
  .dashboard-cell-random,
  .dashboard-cell-heatmap {
    grid-column: 1 / -1;
  }
}
</style>
