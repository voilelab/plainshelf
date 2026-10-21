<template>
  <section class="dashboard-page">
    <header class="dashboard-header">
      <h2>{{ t('dashboard.title') }}</h2>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <template v-else-if="loading">
      <p class="loading-status" :class="{ 'sr-only': !shelfInitializing }" role="status">
        {{ shelfInitializing ? t('dashboard.shelfInitializing') : t('dashboard.loading') }}
      </p>
      <DashboardSkeleton />
    </template>

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
      <RecentlyAdded class="dashboard-cell dashboard-cell-recently-added" :books="recentlyAdded" />
      <TagCloud class="dashboard-cell dashboard-cell-tags" :tag-counts="tagCounts" />
      <RandomBook
        class="dashboard-cell dashboard-cell-random"
        :books="books"
        :started-ids="startedBookIds"
      />
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
import RecentlyAdded from '@/features/dashboard/components/RecentlyAdded.vue';
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
  recentlyAdded,
  startedBookIds,
  fetchDashboardData
} = useDashboardData();

useDocumentTitle(() => [t('dashboard.title'), 'PlainShelf']);

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

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.dashboard-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.dashboard-cell-recent,
.dashboard-cell-stats,
.dashboard-cell-recently-added,
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
  .dashboard-cell-recently-added,
  .dashboard-cell-tags,
  .dashboard-cell-random,
  .dashboard-cell-heatmap {
    grid-column: 1 / -1;
  }
}
</style>
