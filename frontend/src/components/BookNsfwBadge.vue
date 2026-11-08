<template>
  <span v-if="isBookNsfw(book)" class="book-nsfw-badge" :title="t('bookCollection.nsfwBadge.title')">
    {{ t('bookCollection.nsfwBadge.label') }}
  </span>
</template>

<script setup lang="ts">
import { isBookNsfw, type Book } from '@/types/book';
import { useI18n } from '@/i18n';

defineProps<{ book: Pick<Book, 'nsfw' | 'nsfw_folder'> }>();

const { t } = useI18n();
</script>

<style scoped>
.book-nsfw-badge {
  --badge-color: var(--nsfw);
  align-items: center;
  background: color-mix(in srgb, var(--badge-color) 10%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--badge-color) 35%, var(--surface));
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
