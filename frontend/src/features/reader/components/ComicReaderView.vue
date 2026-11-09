<template>
  <section class="comic-reader" data-reader-variant="comic" :data-page-direction="direction">
    <header class="comic-toolbar">
      <button v-if="showBackNavigation" class="button" type="button" @click="emit('back')">{{ t('common.back') }}</button>
      <strong>{{ title }}</strong>
      <span class="comic-count" aria-live="polite">{{ pages.length ? index + 1 : 0 }} / {{ pages.length }}</span>
      <div class="comic-size" role="group" :aria-label="t('reader.comic.pageSize')">
        <select class="input comic-fit" :value="fit" :aria-label="t('reader.comic.pageSize')" @change="setFit(($event.target as HTMLSelectElement).value)">
          <option v-for="option in fitOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <button class="button" type="button" :disabled="zoom <= COMIC_ZOOM_MIN" :aria-label="t('reader.comic.zoomOut')" @click="zoomOut">−</button>
        <button class="button comic-zoom" type="button" :aria-label="t('reader.comic.resetZoom', { zoom })" @click="resetZoom">{{ zoom }}%</button>
        <button class="button" type="button" :disabled="zoom >= COMIC_ZOOM_MAX" :aria-label="t('reader.comic.zoomIn')" @click="zoomIn">+</button>
      </div>
    </header>
    <p v-if="saveError" role="status">{{ t('reader.autosaveFailed') }}</p>
    <div v-if="loading" class="comic-status" role="status">{{ t('reader.loadingContent') }}</div>
    <div v-else-if="error" class="comic-status" role="alert">
      <p>{{ error }}</p>
      <button class="button" type="button" @click="emit('retry')">{{ t('common.retry') }}</button>
    </div>
    <div
      v-else
      ref="viewport"
      class="comic-viewport"
      @click="onClick"
      @touchstart.passive="onTouchStart"
      @touchmove.passive="onTouchMove"
      @touchend.passive="onTouchEnd"
      @touchcancel="cancelTouch"
    >
      <p v-if="!pages.length" class="comic-status">{{ t('reader.comic.empty') }}</p>
      <ComicPageImage
        v-for="pageIndex in windowIndices"
        v-show="pageIndex === displayedIndex"
        :key="`${pageIndex}:${pages[pageIndex].name}`"
        :book-id="bookId"
        :source-id="sourceId"
        :page="pages[pageIndex]"
        :index="pageIndex"
        :viewport="viewportSize"
        :fit="fit"
        :zoom="zoom"
        @ready="pageReady(pageIndex)"
      />
    </div>
    <footer class="comic-navigation">
      <div class="comic-buttons" :class="{ 'comic-buttons-rtl': direction === 'rtl' }">
        <button class="button" type="button" :disabled="loading || !!error || !pages.length || index <= 0" @click="emit('previous')">{{ t('common.prev') }}</button>
        <label class="comic-progress">
          <span>{{ t('reader.progress', { percent }) }}</span>
          <input
            type="range" min="0" :max="Math.max(0, pages.length - 1)" :value="index"
            :disabled="loading || !!error || pages.length < 2" :dir="direction" :aria-label="t('reader.comic.choosePage')"
            @input="emit('select', Number(($event.target as HTMLInputElement).value))"
          />
        </label>
        <button class="button" type="button" :disabled="loading || !!error || !pages.length || index >= pages.length - 1" @click="emit('next')">{{ t('common.next') }}</button>
      </div>
      <span class="comic-hint">{{ t(direction === 'rtl' ? 'reader.comic.hintRtl' : 'reader.comic.hintLtr') }}</span>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import ComicPageImage from './ComicPageImage.vue';
import type { SourcePage } from '@/types/source';
import { useComicPageSize } from '@/features/reader/composables/useComicPageSize';
import { COMIC_ZOOM_MAX, COMIC_ZOOM_MIN } from '@/features/reader/utils/comicPageSize';
import { classifyReaderGesture } from '@/features/reader/utils/mobileReaderGestures';
import { useI18n } from '@/i18n';

const props = defineProps<{
  bookId: string; sourceId: string; title: string; pages: SourcePage[]; index: number;
  direction: 'ltr' | 'rtl'; percent: number; loading: boolean; error: string; saveError: string;
  showBackNavigation: boolean;
}>();
const emit = defineEmits<{ back: []; retry: []; previous: []; next: []; select: [index: number] }>();
const { t } = useI18n();
const viewport = ref<HTMLElement | null>(null);
const displayedIndex = ref(props.index);
const viewportSize = ref({ width: 0, height: 0 });
const { fit, zoom, setFit, zoomIn, zoomOut, resetZoom } = useComicPageSize();
const fitOptions = computed(() => [
  { value: 'fit-height', label: t('reader.comic.fitHeight') },
  { value: 'fit-width', label: t('reader.comic.fitWidth') },
  { value: 'original', label: t('reader.comic.original') }
]);
function measureViewport(): void {
  if (viewport.value) viewportSize.value = { width: viewport.value.clientWidth, height: viewport.value.clientHeight };
}
let resizeObserver: ResizeObserver | null = null;
watch(viewport, (element) => {
  resizeObserver?.disconnect();
  resizeObserver = null;
  if (!element) return;
  measureViewport();
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(measureViewport);
    resizeObserver.observe(element);
  }
}, { flush: 'post' });
function scrollToPageStart(): void {
  const element = viewport.value;
  if (!element) return;
  element.scrollTop = 0;
  element.scrollLeft = props.direction === 'rtl' ? element.scrollWidth - element.clientWidth : 0;
}
watch([displayedIndex, fit], () => { void nextTick(scrollToPageStart); });
function isTextEntry(element: Element | null): boolean {
  return element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement
    || element?.getAttribute('contenteditable') === 'true';
}
function onKeydown(event: KeyboardEvent): void {
  if (event.ctrlKey || event.metaKey || event.altKey || isTextEntry(document.activeElement)) return;
  if (event.key === '+' || event.key === '=') zoomIn();
  else if (event.key === '-') zoomOut();
  else if (event.key === '0') resetZoom();
  else return;
  event.preventDefault();
}
onMounted(() => document.addEventListener('keydown', onKeydown));
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown);
  resizeObserver?.disconnect();
});
function browserZoomed(): boolean {
  return (window.visualViewport?.scale ?? 1) > 1.01;
}
function pansHorizontally(): boolean {
  return !!viewport.value && viewport.value.scrollWidth > viewport.value.clientWidth + 1;
}
const readyPages = new Set<number>();
const windowIndices = computed(() => {
  const indices = new Set<number>();
  for (let i = Math.max(0, props.index - 2); i <= Math.min(props.pages.length - 1, props.index + 2); i++) indices.add(i);
  if (displayedIndex.value >= 0 && displayedIndex.value < props.pages.length) indices.add(displayedIndex.value);
  return [...indices];
});
watch(windowIndices, (indices) => {
  for (const index of readyPages) if (!indices.includes(index)) readyPages.delete(index);
});
watch(() => props.index, (index) => {
  if (readyPages.has(index)) displayedIndex.value = index;
});
function pageReady(index: number): void {
  readyPages.add(index);
  if (props.index !== index) return;
  displayedIndex.value = index;
  void nextTick(scrollToPageStart);
}
function turn(next: boolean): void {
  if (props.loading || props.error) return;
  if (next) { if (props.index < props.pages.length - 1) emit('next'); }
  else if (props.index > 0) emit('previous');
}
function onClick(event: MouseEvent): void {
  if (performance.now() < ignoreClickUntil || !viewport.value || browserZoomed()) return;
  const bounds = viewport.value.getBoundingClientRect();
  const x = (event.clientX - bounds.left) / bounds.width;
  if (x < 0.3 || x > 0.7) turn((x < 0.3) === (props.direction === 'rtl'));
}
let touch: { id: number; x: number; y: number; time: number; maxMovement: number } | null = null;
let ignoreClickUntil = 0;
function cancelTouch(): void { touch = null; }
function onTouchStart(event: TouchEvent): void {
  ignoreClickUntil = performance.now() + 800;
  const first = event.touches.item(0);
  touch = event.touches.length === 1 && first ? { id: first.identifier, x: first.clientX, y: first.clientY, time: performance.now(), maxMovement: 0 } : null;
}
function onTouchMove(event: TouchEvent): void {
  if (event.touches.length !== 1) cancelTouch();
  if (!touch) return;
  const current = Array.from(event.touches).find((item) => item.identifier === touch!.id);
  if (current) touch.maxMovement = Math.max(touch.maxMovement, Math.hypot(current.clientX - touch.x, current.clientY - touch.y));
}
function onTouchEnd(event: TouchEvent): void {
  ignoreClickUntil = performance.now() + 800;
  const start = touch;
  cancelTouch();
  if (!start || event.touches.length) return;
  const end = Array.from(event.changedTouches).find((item) => item.identifier === start.id);
  if (!end || !viewport.value || browserZoomed()) return;
  const dx = end.clientX - start.x;
  const dy = end.clientY - start.y;
  const gesture = classifyReaderGesture({ deltaX: dx, deltaY: dy, maxMovement: Math.max(start.maxMovement, Math.hypot(dx, dy)), durationMs: performance.now() - start.time });
  if (gesture === 'next' || gesture === 'previous') {
    if (!pansHorizontally()) turn((gesture === 'next') === (props.direction === 'ltr'));
  } else if (gesture === 'tap') {
    const bounds = viewport.value.getBoundingClientRect();
    const x = (end.clientX - bounds.left) / bounds.width;
    if (x < 0.3 || x > 0.7) turn((x < 0.3) === (props.direction === 'rtl'));
  }
}
</script>

<style scoped>
.comic-reader {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--comic-bg);
  color: var(--comic-text);
}
.comic-toolbar, .comic-navigation { padding: 8px max(12px, env(safe-area-inset-right)) 8px max(12px, env(safe-area-inset-left)); }
.comic-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; padding-top: max(8px, env(safe-area-inset-top)); }
.comic-toolbar strong { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.comic-count { white-space: nowrap; }
.comic-viewport { flex: 1; min-height: 0; display: grid; overflow: auto; overscroll-behavior: contain; touch-action: pan-x pan-y pinch-zoom; }
.comic-size { display: flex; align-items: center; gap: 4px; }
.comic-size .input { width: auto; padding: 6px 8px; }
.comic-size .button { min-width: 36px; }
.comic-zoom { font-variant-numeric: tabular-nums; }
.comic-status { margin: auto; text-align: center; }
.comic-buttons { display: flex; align-items: center; justify-content: center; gap: 12px; }
.comic-buttons-rtl { flex-direction: row-reverse; }
.comic-buttons .button { min-height: 44px; }
.comic-progress { flex: 1; max-width: 600px; text-align: center; font-size: 0.8rem; }
.comic-progress input { display: block; width: 100%; }
.comic-navigation { padding-bottom: max(8px, env(safe-area-inset-bottom)); }
.comic-hint { display: block; text-align: center; font-size: 0.75rem; color: var(--comic-muted); margin-top: 4px; }
</style>
