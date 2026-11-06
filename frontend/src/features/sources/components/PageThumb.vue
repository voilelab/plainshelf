<template>
  <div class="page-thumb">
    <img v-if="!missing && src && !failed" :src="src" :alt="alt" loading="lazy" draggable="false" @error="handleError">
    <span v-else class="page-thumb-placeholder">{{ missing ? t('sources.pages.missing') : alt }}</span>
  </div>
</template>

<script setup lang="ts">
import { useAssetSrc } from '@/composables/useAssetSrc';
import { useI18n } from '@/i18n';

const props = defineProps<{
  bookId: string;
  sourceId: string;
  name: string;
  alt: string;
  missing?: boolean;
}>();

const { t } = useI18n();
const { src, failed, handleError } = useAssetSrc(
  () => props.bookId,
  () => props.sourceId,
  () => (props.missing ? '' : props.name)
);
</script>

<style scoped>
.page-thumb {
  aspect-ratio: 2 / 3;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 6px;
  background: #f1f5f9;
}

.page-thumb img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.page-thumb-placeholder {
  padding: 6px;
  color: var(--muted);
  font-size: 12px;
  text-align: center;
  word-break: break-all;
}
</style>
