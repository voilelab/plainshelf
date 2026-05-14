<template>
  <article
    class="book-card panel"
    :class="{ 'is-dragging': isDragging }"
    draggable="true"
    @click="emit('select', book.id)"
    @dragstart="onDragStart"
    @dragend="onDragEnd"
  >
    <img :src="coverSrc" :alt="book.title" class="cover" @error="onCoverError" />
    <div class="body">
      <h3 class="title">{{ book.title }}</h3>
      <p class="meta">{{ (book.authors ?? []).join(', ') }}</p>
      <p class="meta layer-path" aria-label="Layer path">
        <template v-for="(segment, index) in layerSegments" :key="`${book.id}-${segment}-${index}`">
          <span>{{ segment }}</span>
          <span v-if="index < layerSegments.length - 1" class="sep"> / </span>
        </template>
      </p>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import bookcover from '../assets/bookcover.svg';
import type { Book } from '../types/book';
import { getLayerPath, layerPathLabel } from '../utils/layers';

const props = defineProps<{ book: Book }>();

const emit = defineEmits<{
  (event: 'select', id: string): void;
}>();

const hasCoverLoadError = ref(false);
const isDragging = ref(false);
const dragPreviewEl = ref<HTMLElement | null>(null);

const coverSrc = computed(() => {
  if (hasCoverLoadError.value) {
    return bookcover;
  }
  return props.book.cover_url || bookcover;
});

const layerPath = computed(() => getLayerPath(props.book));
const layerSegments = computed(() => {
  if (!layerPath.value) {
    return [layerPathLabel(layerPath.value)];
  }
  return layerPath.value.split(' / ');
});

watch(
  () => props.book.cover_url,
  () => {
    hasCoverLoadError.value = false;
  }
);

function onCoverError(): void {
  hasCoverLoadError.value = true;
}

function createDragPreview(book: Book): HTMLElement {
  const el = document.createElement('div');
  const coverEl = document.createElement('img');
  const titleEl = document.createElement('div');
  const authorEl = document.createElement('div');

  el.className = 'book-drag-preview';

  coverEl.src = book.cover_url || bookcover;
  coverEl.alt = '';
  coverEl.className = 'book-drag-preview-cover';

  titleEl.textContent = book.title;
  titleEl.className = 'book-drag-preview-title';

  authorEl.textContent = (book.authors ?? []).join(', ');
  authorEl.className = 'book-drag-preview-author';

  el.append(coverEl, titleEl, authorEl);
  return el;
}

function cleanupDragPreview(): void {
  dragPreviewEl.value?.remove();
  dragPreviewEl.value = null;
}

function onDragStart(event: DragEvent): void {
  isDragging.value = true;
  cleanupDragPreview();
  event.dataTransfer?.setData('application/x-txtlib-book-id', props.book.id);
  event.dataTransfer?.setData('text/plain', props.book.id);
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    const preview = createDragPreview(props.book);
    document.body.appendChild(preview);
    dragPreviewEl.value = preview;
    event.dataTransfer.setDragImage(preview, 60, 80);
  }
}

function onDragEnd(): void {
  isDragging.value = false;
  cleanupDragPreview();
}

onBeforeUnmount(() => {
  cleanupDragPreview();
});
</script>

<style scoped>
.book-card {
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.12s ease;
}

.book-card:hover {
  transform: translateY(-2px);
}

.book-card.is-dragging {
  opacity: 0.35;
  transform: scale(0.92);
  transform-origin: center;
  transition: opacity 120ms ease, transform 120ms ease;
}

.cover {
  width: 100%;
  height: 230px;
  object-fit: cover;
  background: #f2f2f2;
}

.body {
  padding: 10px 12px 14px;
}

.title {
  margin: 0 0 6px;
  font-size: 16px;
  line-height: 1.3;
}

.layer-path {
  margin: 6px 0 0;
}

.sep {
  opacity: 0.75;
}

:global(.book-drag-preview) {
  position: fixed;
  top: -1000px;
  left: -1000px;
  width: 120px;
  height: 160px;
  padding: 8px;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.18);
  overflow: hidden;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
  box-sizing: border-box;
}

:global(.book-drag-preview-cover) {
  width: 100%;
  height: 96px;
  object-fit: cover;
  border-radius: 8px;
  background: #f2f2f2;
}

:global(.book-drag-preview-title) {
  color: #0f172a;
  display: -webkit-box;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.2;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}

:global(.book-drag-preview-author) {
  color: #475569;
  font-size: 11px;
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
