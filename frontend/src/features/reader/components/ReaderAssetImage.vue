<template>
  <figure class="reader-md-figure">
    <img
      v-if="src && !failed"
      class="reader-md-image"
      :src="src"
      :alt="alt"
      @error="handleError"
    />
    <p v-else-if="failed" class="reader-md-image-fallback">{{ fallbackText }}</p>
    <div v-else class="reader-md-image-placeholder" aria-hidden="true" />
    <figcaption v-if="alt && !failed" class="reader-md-figcaption">{{ alt }}</figcaption>
  </figure>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useAssetSrc } from '@/composables/useAssetSrc';
import { useI18n } from '@/i18n';

const props = defineProps<{
  bookId: string;
  sourceId: string;
  name: string;
  alt: string;
}>();

const { t } = useI18n();

const { src, failed, handleError } = useAssetSrc(
  () => props.bookId,
  () => props.sourceId,
  () => props.name
);

// An illustration that cannot be loaded is reported in place rather than
// leaving a gap: the alt text is what the author wrote to stand in for it, and
// the generic label only appears when there is no alt text to show.
const fallbackText = computed(() => props.alt || t('reader.imageUnavailable'));
</script>

<style scoped>
.reader-md-figure {
  margin: 1.5rem 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.reader-md-image {
  display: block;
  max-width: 100%;
  height: auto;
}

.reader-md-image-placeholder {
  width: 100%;
  min-height: 2rem;
}

.reader-md-image-fallback,
.reader-md-figcaption {
  margin: 0;
  font-size: 0.9em;
  opacity: 0.75;
  text-align: center;
}
</style>
