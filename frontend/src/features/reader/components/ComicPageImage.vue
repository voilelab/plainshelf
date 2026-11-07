<template>
  <div class="comic-page-image">
    <p v-if="page.missing || failed" class="comic-page-unavailable" role="status">
      {{ t('reader.comic.unavailable', { page: index + 1 }) }}
    </p>
    <img
      v-else-if="src"
      :src="src"
      :alt="t('reader.comic.page', { page: index + 1 })"
      :style="sizeStyle"
      draggable="false"
      @load="onLoad"
      @error="handleError"
    />
    <p v-else role="status">{{ t('reader.loadingContent') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useAssetSrc } from '@/composables/useAssetSrc';
import { useI18n } from '@/i18n';
import type { SourcePage } from '@/types/source';
import { comicPageSize, type ComicFit } from '@/features/reader/utils/comicPageSize';

const props = defineProps<{
  bookId: string; sourceId: string; page: SourcePage; index: number;
  viewport: { width: number; height: number }; fit: ComicFit; zoom: number;
}>();
const emit = defineEmits<{ ready: [] }>();
const { t } = useI18n();
const { src, failed, handleError } = useAssetSrc(
  () => props.bookId,
  () => props.sourceId,
  () => props.page.missing ? '' : props.page.name
);
const natural = ref({ width: 0, height: 0 });
const sizeStyle = computed(() => {
  const size = comicPageSize(natural.value, props.viewport, props.fit, props.zoom);
  return size ? { width: `${size.width}px`, height: `${size.height}px` } : undefined;
});
function onLoad(event: Event): void {
  const img = event.target as HTMLImageElement;
  natural.value = { width: img.naturalWidth, height: img.naturalHeight };
  emit('ready');
}
watch(failed, (value) => { if (value) emit('ready'); }, { immediate: true });
</script>

<style scoped>
.comic-page-image {
  display: grid;
  color: #eee;
}

.comic-page-image > * {
  margin: auto;
}

.comic-page-image img {
  display: block;
  max-width: none;
  user-select: none;
}

.comic-page-unavailable {
  padding: 24px;
  text-align: center;
}
</style>
