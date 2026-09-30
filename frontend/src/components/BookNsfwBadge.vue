<template>
  <span v-if="isBookNsfw(book)" class="book-nsfw-badge" :title="t('bookCollection.nsfwBadge.title')">
    {{ t('bookCollection.nsfwBadge.label') }}
  </span>
</template>

<script setup lang="ts">
import { isBookNsfw, type Book } from '@/types/book';
import { useI18n } from '@/i18n';

// Rendered only where the mark is actually visible, which is only while the
// server is serving marked books at all: with "show adult content" off they are
// filtered out before any listing reaches the client. So the badge answers the
// question that setting raises — which of these disappear when I turn it back
// off — rather than labelling books for their own sake.
//
// Takes the book rather than a boolean because the mark has two halves and
// isBookNsfw is the one place that adds them.
defineProps<{ book: Pick<Book, 'nsfw' | 'nsfw_folder'> }>();

const { t } = useI18n();
</script>

<style scoped>
.book-nsfw-badge {
  --badge-color: #b3436b;
  align-items: center;
  background: color-mix(in srgb, var(--badge-color) 10%, white);
  border: 1px solid color-mix(in srgb, var(--badge-color) 35%, white);
  border-radius: 999px;
  color: color-mix(in srgb, var(--badge-color) 80%, var(--text));
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.4;
  max-width: 100%;
  padding: 2px 8px;
  white-space: nowrap;
}
</style>
