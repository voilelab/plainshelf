<template>
  <div class="detail-main">
    <div class="detail-heading">
      <LayerBreadcrumb :layers="book.layers" />
      <h2 class="detail-title">{{ book.title }}</h2>
    </div>

    <div class="meta-list" role="list" aria-label="Book metadata">
      <div v-for="row in metadataRows" :key="row.label" class="meta-row" role="listitem">
        <p class="meta-label">{{ row.label }}</p>
        <p v-if="!row.href" class="meta-value" :class="row.className">{{ row.value }}</p>
        <p v-else class="meta-value" :class="row.className">
          <a class="meta-link" :href="row.href" target="_blank" rel="noreferrer">{{ row.value }}</a>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import LayerBreadcrumb from './LayerBreadcrumb.vue';
import type { Book, ReadingProgress } from '../types/book';
import { formatLanguage } from '../utils/language';

const props = defineProps<{
  book: Book;
  progress?: ReadingProgress | null;
}>();

interface MetadataRow {
  label: string;
  value: string;
  href?: string;
  className?: string;
}

function formatList(values: string[]): string {
  return values.length > 0 ? values.join(', ') : '-';
}

function formatTimestamp(value?: string): string {
  if (!value) {
    return '-';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString();
}

const metadataRows = computed<MetadataRow[]>(() => {
  const rows: MetadataRow[] = [
    { label: 'Authors', value: formatList(props.book.authors) },
    { label: 'Format', value: props.book.format?.trim() || '-' },
    { label: 'Language', value: formatLanguage(props.book.language) },
    { label: 'Tags', value: formatList(props.book.tags) },
    { label: 'Published At', value: formatTimestamp(props.book.published_at) },
    {
      label: 'Comment',
      value: props.book.comment?.trim() || '-',
      className: 'comment-text'
    }
  ];

  return rows;
});
</script>

<style scoped>
.detail-main {
  display: grid;
  gap: 18px;
}

.detail-heading {
  display: grid;
  gap: 10px;
}

.detail-title {
  margin: 0;
  font-size: clamp(24px, 3vw, 30px);
  line-height: 1.08;
  font-weight: 800;
  letter-spacing: -0.01em;
}

.meta-list {
  display: grid;
  border-top: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
}

.meta-row {
  min-height: 64px;
  display: grid;
  grid-template-columns: minmax(150px, 210px) minmax(0, 1fr);
  align-items: center;
  gap: 14px;
  padding: 14px 0;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.meta-label,
.meta-value {
  margin: 0;
}

.meta-label {
  font-size: clamp(18px, 2vw, 20px);
  font-weight: 700;
  color: color-mix(in srgb, var(--text) 80%, #1f2937);
}

.meta-value {
  font-size: clamp(18px, 1.7vw, 20px);
  line-height: 1.35;
  color: var(--text);
}

.comment-text {
  white-space: pre-wrap;
}

.meta-link {
  color: inherit;
  text-decoration: underline;
  text-underline-offset: 0.18em;
  word-break: break-all;
}
</style>