<template>
  <div class="tag-cloud panel">
    <h3 class="tag-cloud-title">{{ t('dashboard.tags.title') }}</h3>
    <p v-if="entries.length === 0" class="tag-cloud-empty">{{ t('dashboard.tags.empty') }}</p>
    <div v-else class="tag-cloud-body">
      <RouterLink
        v-for="entry in entries"
        :key="entry.tag"
        class="tag-chip"
        :style="{ fontSize: `${entry.fontSize}px` }"
        :title="`${entry.tag} (${entry.count})`"
        :to="entry.to"
      >
        {{ entry.tag }}
      </RouterLink>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { RouteLocationRaw } from 'vue-router';
import { useI18n } from '@/i18n';
import { serializeFilterField, serializeFilterFieldOp, filterFieldOpKey } from '@/utils/bookFilters/codec';

const props = defineProps<{
  tagCounts: Record<string, number>;
}>();

const { t } = useI18n();

const MIN_FONT_SIZE = 12;
const MAX_FONT_SIZE = 28;

// A chip links to the library with this tag already applied. The token grammar
// (`eq:<tag>` plus the `tagsOp` combinator) is the tags filter's own, so the
// landing page parses it back into an active filter chip rather than ignoring a
// bare `?tags=<tag>`; building it through the serializers keeps it in step if
// that grammar ever changes.
function tagLinkTo(tag: string): RouteLocationRaw {
  return {
    path: '/books',
    query: {
      tags: serializeFilterField({ kind: 'eq', value: tag }),
      [filterFieldOpKey('tags')]: serializeFilterFieldOp('all')
    }
  };
}

// Log scale: raw counts vary widely (a handful of tags vs. one dominant tag),
// and a linear map would make everything but the top tag look identical.
const entries = computed(() => {
  const pairs = Object.entries(props.tagCounts).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  if (pairs.length === 0) {
    return [];
  }

  const logCounts = pairs.map(([, count]) => Math.log(count + 1));
  const minLog = Math.min(...logCounts);
  const maxLog = Math.max(...logCounts);
  const range = maxLog - minLog;

  return pairs.map(([tag, count], index) => {
    const ratio = range === 0 ? 1 : (logCounts[index] - minLog) / range;
    const fontSize = Math.round(MIN_FONT_SIZE + ratio * (MAX_FONT_SIZE - MIN_FONT_SIZE));
    return { tag, count, fontSize, to: tagLinkTo(tag) };
  });
});
</script>

<style scoped>
.tag-cloud {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 18px;
}

.tag-cloud-title {
  color: var(--text);
  font-size: 14px;
  font-weight: 700;
  margin: 0;
}

.tag-cloud-empty {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}

.tag-cloud-body {
  align-items: baseline;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
}

.tag-chip {
  color: var(--accent);
  cursor: pointer;
  font-weight: 600;
  line-height: 1.2;
  text-decoration: none;
}

.tag-chip:hover,
.tag-chip:focus-visible {
  text-decoration: underline;
}
</style>
