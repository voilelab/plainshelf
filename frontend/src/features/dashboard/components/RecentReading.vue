<template>
  <section class="recent-reading panel">
    <div class="recent-reading-header">
      <h3 class="recent-reading-title">{{ t('dashboard.recentReading.title') }}</h3>
      <RouterLink v-if="items.length > 0" class="recent-reading-all" to="/read-history">
        {{ t('dashboard.recentReading.viewAll') }}
      </RouterLink>
    </div>

    <div v-if="items.length === 0" class="recent-reading-empty">
      <p class="recent-reading-empty-text">{{ t('readHistory.empty') }}</p>
      <RouterLink class="button" to="/books">
        {{ t('dashboard.recentReading.browse') }}
      </RouterLink>
    </div>

    <ul v-else class="recent-reading-list">
      <li v-for="item in items" :key="item.book.id" class="recent-reading-item">
        <RouterLink v-slot="{ href }" custom :to="readerRoutePath(item.book.id)">
          <a class="recent-reading-card" :href="href" @click="onReaderLinkClick($event, item.book.id)">
            <BookCoverImg
              :book-id="item.book.id"
              :cover-url="item.book.cover_url"
              :alt="item.book.title"
              class="recent-reading-cover"
            />
            <p class="recent-reading-book-title" :title="item.book.title">{{ item.book.title }}</p>

            <div
              v-if="item.percent !== null"
              class="recent-reading-progress"
              role="progressbar"
              :aria-valuenow="item.percent"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <div class="recent-reading-progress-track">
                <div class="recent-reading-progress-fill" :style="{ width: `${item.percent}%` }" />
              </div>
              <span class="recent-reading-progress-value">{{ item.percent }}%</span>
            </div>

            <p v-if="item.lastReadAt !== null" class="recent-reading-time">
              {{ formatRelativeTime(item.lastReadAt, locale, now) }}
            </p>
          </a>
        </RouterLink>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import BookCoverImg from '@/components/BookCoverImg.vue';
import { formatRelativeTime } from '@/utils/date';
import { useI18n } from '@/i18n';
import { readerRoutePath, useReaderLaunch } from '@/composables/useReaderLaunch';
import type { RecentReadingItem } from '@/features/dashboard/composables/useDashboardData';

defineProps<{
  items: RecentReadingItem[];
}>();

const { t, locale } = useI18n();

// The reader entry follows the device-local "reader launch preference" — a new
// tab / standalone reader on 'new-reader', in-place navigation on 'in-window' —
// instead of the plain in-window RouterLink it used to be, so the home cards
// match the library and book-detail read actions.
const { onReaderLinkClick } = useReaderLaunch();

// Captured once when the section mounts: the relative labels are a snapshot of
// the moment the dashboard was opened, not a ticking clock.
const now = Date.now();
</script>

<style scoped>
.recent-reading {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px 18px;
}

.recent-reading-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.recent-reading-title {
  color: var(--text);
  font-size: 14px;
  font-weight: 700;
  margin: 0;
}

.recent-reading-all {
  color: var(--muted);
  font-size: 13px;
}

.recent-reading-empty {
  align-items: flex-start;
  color: var(--muted);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.recent-reading-empty-text {
  font-size: 13px;
  margin: 0;
}

.recent-reading-list {
  display: grid;
  gap: 14px;
  grid-auto-columns: minmax(120px, 160px);
  grid-auto-flow: column;
  justify-content: start;
  list-style: none;
  margin: 0;
  overflow-x: auto;
  padding: 0 0 4px;
}

.recent-reading-item {
  min-width: 0;
}

.recent-reading-card {
  color: inherit;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  text-decoration: none;
}

.recent-reading-cover {
  aspect-ratio: 2 / 3;
  background: #f2f2f2;
  border-radius: 6px;
  object-fit: cover;
  width: 100%;
}

.recent-reading-book-title {
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-reading-progress {
  align-items: center;
  display: flex;
  gap: 6px;
}

.recent-reading-progress-track {
  background: var(--border, #e0e0e0);
  border-radius: 999px;
  flex: 1 1 auto;
  height: 4px;
  overflow: hidden;
}

.recent-reading-progress-fill {
  background: var(--primary, #4a7);
  height: 100%;
}

.recent-reading-progress-value {
  color: var(--muted);
  flex: 0 0 auto;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.recent-reading-time {
  color: var(--muted);
  font-size: 11px;
  margin: 0;
}
</style>
