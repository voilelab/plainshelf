<template>
  <div ref="rootEl" class="page-thumb">
    <img v-if="!missing && src && !failed" :src="src" :alt="alt" loading="lazy" draggable="false" @error="handleError">
    <span v-else class="page-thumb-placeholder">{{ missing ? t('sources.pages.missing') : alt }}</span>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useAssetSrc } from '@/composables/useAssetSrc';
import { useI18n } from '@/i18n';

const NEARBY_MARGIN = '600px 0px';

const props = defineProps<{
  bookId: string;
  sourceId: string;
  name: string;
  alt: string;
  missing?: boolean;
  scrollRoot?: HTMLElement | null;
}>();

const { t } = useI18n();
const rootEl = ref<HTMLElement | null>(null);
const nearby = ref(typeof IntersectionObserver === 'undefined');
let observer: IntersectionObserver | null = null;

const { src, failed, handleError } = useAssetSrc(
  () => props.bookId,
  () => props.sourceId,
  () => (props.missing || !nearby.value ? '' : props.name)
);

onMounted(() => {
  if (nearby.value || !rootEl.value) return;
  observer = new IntersectionObserver((entries) => {
    const entry = entries[entries.length - 1];
    if (entry) nearby.value = entry.isIntersecting;
  }, { root: props.scrollRoot ?? null, rootMargin: NEARBY_MARGIN });
  observer.observe(rootEl.value);
});

onBeforeUnmount(() => observer?.disconnect());
</script>

<style scoped>
.page-thumb {
  aspect-ratio: 2 / 3;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 6px;
  background: var(--surface-sunken);
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
