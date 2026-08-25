<template>
  <div class="reading-heatmap panel">
    <h3 class="reading-heatmap-title">{{ t('dashboard.heatmap.title') }}</h3>

    <div class="reading-heatmap-grid">
      <span
        v-for="cell in cells"
        :key="cell.date"
        class="heatmap-cell"
        :class="`level-${cell.level}`"
        :title="cellLabel(cell)"
        :aria-label="cellLabel(cell)"
      ></span>
    </div>

    <div class="reading-heatmap-legend">
      <span class="legend-label">{{ t('dashboard.heatmap.legendLess') }}</span>
      <span v-for="level in [0, 1, 2, 3, 4]" :key="level" class="heatmap-cell" :class="`level-${level}`"></span>
      <span class="legend-label">{{ t('dashboard.heatmap.legendMore') }}</span>
    </div>

    <p v-if="isEmpty" class="reading-heatmap-empty">{{ t('dashboard.heatmap.empty') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from '@/i18n';

const props = defineProps<{
  /** Map of "YYYY-MM-DD" -> seconds spent reading that day. */
  data: Record<string, number>;
}>();

const { t } = useI18n();

// An upper bound on how many days the strip covers. The cells now flow to fill
// the container width (a wide band that grows shorter as it widens) rather than
// forming fixed Sun-Sat week columns, so this only caps the total, and the CSS
// grid decides how many fit per row at the current width.
const WEEKS_TO_SHOW = 20;

interface HeatmapCell {
  date: string;
  seconds: number;
  level: 0 | 1 | 2 | 3 | 4;
}

function levelFor(seconds: number): 0 | 1 | 2 | 3 | 4 {
  if (seconds <= 0) return 0;
  if (seconds < 15 * 60) return 1;
  if (seconds < 30 * 60) return 2;
  if (seconds < 60 * 60) return 3;
  return 4;
}

function toIsoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

const cells = computed<HeatmapCell[]>(() => {
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  const start = new Date(today);
  start.setDate(start.getDate() - (WEEKS_TO_SHOW * 7 - 1));

  const list: HeatmapCell[] = [];
  const cursor = new Date(start);
  while (cursor <= today) {
    const iso = toIsoDate(cursor);
    const seconds = props.data[iso] ?? 0;
    list.push({ date: iso, seconds, level: levelFor(seconds) });
    cursor.setDate(cursor.getDate() + 1);
  }
  return list;
});

const isEmpty = computed(() => Object.keys(props.data).length === 0);

function cellLabel(cell: HeatmapCell): string {
  const minutes = Math.round(cell.seconds / 60);
  return t('dashboard.heatmap.cellLabel', { date: cell.date, minutes });
}
</script>

<style scoped>
.reading-heatmap {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 18px;
  width: 100%;
}

.reading-heatmap-title {
  color: var(--text);
  font-size: 14px;
  font-weight: 700;
  margin: 0;
}

.reading-heatmap-grid {
  display: grid;
  gap: 3px;
  /* Cells stretch to fill the container width: as many ~11px columns as fit,
     each growing to an equal share of the row. A wide viewport gets a short,
     full-width band instead of leaving two-thirds of the row empty. */
  grid-template-columns: repeat(auto-fill, minmax(11px, 1fr));
  width: 100%;
}

.heatmap-cell {
  aspect-ratio: 1 / 1;
  border-radius: 2px;
  min-height: 11px;
  width: 100%;
}

.reading-heatmap-legend .heatmap-cell {
  aspect-ratio: auto;
  height: 11px;
  min-height: 0;
  width: 11px;
}

.heatmap-cell.level-0 {
  background: var(--border);
}

.heatmap-cell.level-1 {
  background: #9be9a8;
}

.heatmap-cell.level-2 {
  background: #40c463;
}

.heatmap-cell.level-3 {
  background: #30a14e;
}

.heatmap-cell.level-4 {
  background: #216e39;
}

.reading-heatmap-legend {
  align-items: center;
  display: flex;
  gap: 4px;
}

.legend-label {
  color: var(--muted);
  font-size: 11px;
}

.reading-heatmap-empty {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}
</style>
