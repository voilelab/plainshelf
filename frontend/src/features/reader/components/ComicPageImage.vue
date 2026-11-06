<template>
  <div class="comic-page-image">
    <p v-if="page.missing || failed" class="comic-page-unavailable" role="status">
      {{ t('reader.comic.unavailable', { page: index + 1 }) }}
    </p>
    <img
      v-else-if="src"
      :src="src"
      :alt="t('reader.comic.page', { page: index + 1 })"
      draggable="false"
      @load="emit('ready')"
      @error="handleError"
    />
    <p v-else role="status">{{ t('reader.loadingContent') }}</p>
  </div>
</template>

<script setup lang="ts">
import { watch } from 'vue';
import { useAssetSrc } from '@/composables/useAssetSrc';
import { useI18n } from '@/i18n';
import type { SourcePage } from '@/types/source';

const props = defineProps<{ bookId: string; sourceId: string; page: SourcePage; index: number }>();
const emit = defineEmits<{ ready: [] }>();
const { t } = useI18n();
const { src, failed, handleError } = useAssetSrc(
  () => props.bookId,
  () => props.sourceId,
  () => props.page.missing ? '' : props.page.name
);
watch(failed, (value) => { if (value) emit('ready'); }, { immediate: true });
</script>

<style scoped>
.comic-page-image {
  position: absolute;
  inset: 0;
  display: flex;
  justify-content: center;
  align-items: center;
  color: #eee;
}

.comic-page-image img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  user-select: none;
}

.comic-page-unavailable {
  padding: 24px;
  text-align: center;
}
</style>
