<template>
  <article class="duplicate-book-row">
    <img :src="coverSrc" :alt="book.title" class="duplicate-cover" @error="onCoverError" />

    <div class="duplicate-meta">
      <h4 class="duplicate-title">{{ book.title }}</h4>
      <p class="duplicate-layer">{{ layerLabel }}</p>
    </div>

    <button type="button" class="button duplicate-open" @click="openBook">Open</button>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import type { Book } from '../types/book';
import { getLayerPath, layerPathLabel } from '../utils/layers';
import bookcover from '../assets/bookcover.svg';

const props = defineProps<{
  book: Book;
}>();

const router = useRouter();
const hasCoverLoadError = ref(false);

const coverSrc = computed(() => {
  if (hasCoverLoadError.value) {
    return bookcover;
  }
  return props.book.cover_url || bookcover;
});

const layerLabel = computed(() => layerPathLabel(getLayerPath(props.book)));

watch(
  () => props.book.cover_url,
  () => {
    hasCoverLoadError.value = false;
  }
);

function onCoverError(): void {
  hasCoverLoadError.value = true;
}

function openBook(): void {
  void router.push(`/books/${props.book.id}`);
}
</script>

<style scoped>
.duplicate-book-row {
  align-items: center;
  border: 1px solid #e4ebf3;
  border-radius: 10px;
  display: grid;
  gap: 12px;
  grid-template-columns: auto minmax(0, 1fr) auto;
  padding: 10px 12px;
}

.duplicate-cover {
  width: 42px;
  height: 62px;
  border-radius: 8px;
  border: 1px solid var(--border);
  object-fit: cover;
  background: #f4f7fb;
}

.duplicate-meta {
  min-width: 0;
}

.duplicate-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.3;
}

.duplicate-layer {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.3;
}

.duplicate-open {
  font-size: 13px;
  font-weight: 600;
  padding: 6px 12px;
}

@media (max-width: 700px) {
  .duplicate-book-row {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .duplicate-open {
    grid-column: 1 / -1;
    justify-self: end;
  }
}
</style>
