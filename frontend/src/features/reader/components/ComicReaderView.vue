<template>
  <section class="comic-reader" data-reader-variant="comic" :data-page-direction="direction">
    <header class="comic-toolbar">
      <button v-if="showBackNavigation" class="button" type="button" @click="emit('back')">{{ t('common.back') }}</button>
      <strong>{{ title }}</strong>
      <span class="comic-count" aria-live="polite">{{ pages.length ? index + 1 : 0 }} / {{ pages.length }}</span>
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
import { computed, ref, watch } from 'vue';
import ComicPageImage from './ComicPageImage.vue';
import type { SourcePage } from '@/types/source';
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
  if (props.index === index) displayedIndex.value = index;
}
function turn(next: boolean): void {
  if (props.loading || props.error) return;
  if (next) { if (props.index < props.pages.length - 1) emit('next'); }
  else if (props.index > 0) emit('previous');
}
function onClick(event: MouseEvent): void {
  if (performance.now() < ignoreClickUntil || !viewport.value) return;
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
  if (!end || !viewport.value) return;
  const dx = end.clientX - start.x;
  const dy = end.clientY - start.y;
  const gesture = classifyReaderGesture({ deltaX: dx, deltaY: dy, maxMovement: Math.max(start.maxMovement, Math.hypot(dx, dy)), durationMs: performance.now() - start.time });
  if (gesture === 'next' || gesture === 'previous') {
    turn((gesture === 'next') === (props.direction === 'ltr'));
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
  background: #181818;
  color: #eee;
}
.comic-toolbar, .comic-navigation { padding: 8px max(12px, env(safe-area-inset-right)) 8px max(12px, env(safe-area-inset-left)); }
.comic-toolbar { display: flex; align-items: center; gap: 12px; padding-top: max(8px, env(safe-area-inset-top)); }
.comic-toolbar strong { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.comic-count { white-space: nowrap; }
.comic-viewport { flex: 1; min-height: 0; position: relative; touch-action: pan-y pinch-zoom; }
.comic-status { margin: auto; text-align: center; }
.comic-buttons { display: flex; align-items: center; justify-content: center; gap: 12px; }
.comic-buttons-rtl { flex-direction: row-reverse; }
.comic-buttons .button { min-height: 44px; }
.comic-progress { flex: 1; max-width: 600px; text-align: center; font-size: 0.8rem; }
.comic-progress input { display: block; width: 100%; }
.comic-navigation { padding-bottom: max(8px, env(safe-area-inset-bottom)); }
.comic-hint { display: block; text-align: center; font-size: 0.75rem; color: #bbb; margin-top: 4px; }
</style>
