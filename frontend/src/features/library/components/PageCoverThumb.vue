<template>
  <button
    ref="root"
    class="page-cover-thumb"
    type="button"
    :aria-pressed="selected ? 'true' : 'false'"
    :aria-label="t('reader.comic.page', { page: index + 1 })"
    :disabled="page.missing || disabled"
    @click="emit('select')"
  >
    <span class="page-cover-thumb-frame">
      <img v-if="src && !failed" :src="src" alt="" decoding="async" @error="handleError" />
      <span v-else-if="page.missing || failed" class="page-cover-thumb-missing">{{ t('bookDetail.cover.fromPage.missing') }}</span>
    </span>
    <span class="page-cover-thumb-label">{{ index + 1 }}</span>
  </button>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useAssetSrc } from '@/composables/useAssetSrc';
import { useI18n } from '@/i18n';
import type { SourcePage } from '@/types/source';

const props = defineProps<{
  bookId: string;
  sourceId: string;
  page: SourcePage;
  index: number;
  selected: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{ select: [] }>();
const { t } = useI18n();
const root = ref<HTMLElement | null>(null);
const visible = ref(typeof IntersectionObserver === 'undefined');
let observer: IntersectionObserver | null = null;

const { src, failed, handleError } = useAssetSrc(
  () => props.bookId,
  () => props.sourceId,
  () => (visible.value && !props.page.missing ? props.page.name : '')
);

onMounted(() => {
  if (visible.value || !root.value) return;
  observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) {
      visible.value = true;
      observer?.disconnect();
      observer = null;
    }
  }, { rootMargin: '200px' });
  observer.observe(root.value);
});

onBeforeUnmount(() => observer?.disconnect());
</script>

<style scoped>
.page-cover-thumb {
  display: grid;
  gap: 4px;
  padding: 4px;
  border: 2px solid transparent;
  border-radius: 8px;
  background: none;
  color: var(--text-subtle);
  cursor: pointer;
  font-size: 12px;
}

.page-cover-thumb[aria-pressed='true'] {
  border-color: var(--accent);
  color: var(--text);
}

.page-cover-thumb:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.page-cover-thumb-frame {
  display: grid;
  place-items: center;
  aspect-ratio: 2 / 3;
  overflow: hidden;
  border-radius: 4px;
  background: var(--surface-sunken);
}

.page-cover-thumb-frame img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.page-cover-thumb-missing {
  padding: 4px;
  text-align: center;
}

.page-cover-thumb-label {
  text-align: center;
  font-variant-numeric: tabular-nums;
}
</style>
