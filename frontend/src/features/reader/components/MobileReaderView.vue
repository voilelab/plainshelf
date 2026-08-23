<template>
  <section
    ref="pageRef"
    class="mobile-reader-page"
    data-reader-variant="mobile"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @pointercancel="onPointerCancel"
    @touchstart.passive="onTouchStart"
    @touchmove.passive="onTouchMove"
    @touchend.passive="onTouchEnd"
    @touchcancel="resetPointer"
  >
    <main class="mobile-reader-main">
      <div v-if="loading" class="loading mobile-reader-status">{{ t('reader.loadingContent') }}</div>
      <div v-else-if="error" class="error mobile-reader-status mobile-reader-error" role="alert">
        <p>{{ error }}</p>
        <button class="button" type="button" @click="emit('retry')">{{ t('common.retry') }}</button>
      </div>

      <article v-else class="mobile-reader-document">
        <p v-if="saveError" class="mobile-reader-warning" role="status">
          {{ t('reader.autosaveFailed') }}
        </p>
        <ReaderContent
          class="mobile-reader-content"
          :book-id="bookId"
          :source-id="sourceId"
          :book-format="bookFormat"
          :section="currentSection"
          @scroll="emit('scroll')"
          @ready="emit('readerReady', $event)"
        />
      </article>
    </main>

    <Transition name="reader-chrome">
      <header v-if="chromeVisible" class="mobile-reader-topbar">
        <RouterLink :to="`/books/${bookId}`" class="mobile-reader-back">{{ t('reader.backToDetail') }}</RouterLink>
        <div class="mobile-reader-chapter">
          <strong>{{ currentSection?.title || title || bookId }}</strong>
          <span>{{ currentSectionIndex + 1 }} / {{ sections.length }}</span>
        </div>
        <span class="mobile-reader-progress">{{ progressPercent }}%</span>
      </header>
    </Transition>

    <Transition name="reader-chrome">
      <div
        v-if="chromeVisible"
        class="mobile-reader-toolbar"
        role="toolbar"
        :aria-label="t('reader.actionsLabel')"
      >
        <button
          class="button mobile-reader-tool mobile-reader-font-tool"
          type="button"
          :aria-label="t('reader.decreaseFontSize')"
          :disabled="isAtMinFontSize"
          @click="emit('decreaseFontSize')"
        >
          A−
        </button>
        <button
          class="button mobile-reader-tool mobile-reader-font-tool"
          type="button"
          :aria-label="t('reader.increaseFontSize')"
          :disabled="isAtMaxFontSize"
          @click="emit('increaseFontSize')"
        >
          A+
        </button>
        <button
          class="button mobile-reader-tool mobile-reader-font-tool"
          type="button"
          :aria-label="t('reader.chooseFont')"
          @click="emit('openFontModal')"
        >
          Aa
        </button>
        <button
          class="button mobile-reader-tool"
          type="button"
          :aria-label="t('reader.showChapters')"
          :disabled="sections.length === 0"
          @click="emit('openChapterModal')"
        >
          <Icon name="menu" />
        </button>
      </div>
    </Transition>

    <Transition name="reader-message">
      <p v-if="showGestureHint" class="mobile-reader-hint" role="status">{{ t('reader.mobile.gestureHint') }}</p>
    </Transition>
    <p class="mobile-reader-live" aria-live="polite">{{ boundaryMessage }}</p>
    <Transition name="reader-message">
      <p v-if="boundaryMessage" class="mobile-reader-boundary" aria-hidden="true">{{ boundaryMessage }}</p>
    </Transition>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import Icon from '@/components/Icon.vue';
import { useWriteAccess } from '@/composables/useWriteAccess';
import ReaderContent from '@/features/reader/components/ReaderContent.vue';
import {
  classifyReaderGesture,
  isInReaderCenter,
  isReaderInteractiveTarget
} from '@/features/reader/utils/mobileReaderGestures';
import type { ReaderSection } from '@/types/book';
import { useI18n } from '@/i18n';

const GESTURE_HINT_STORAGE_KEY = 'reader-mobile-gesture-hint-seen';
const GESTURE_HINT_DURATION_MS = 4_000;
const BOUNDARY_MESSAGE_DURATION_MS = 1_500;

const props = defineProps<{
  bookId: string;
  title: string;
  bookFormat: string;
  sourceId: string;
  sections: ReaderSection[];
  currentSectionIndex: number;
  currentSection: ReaderSection | null;
  progressPercent: number;
  loading: boolean;
  error: string;
  saveError: string;
  isAtMinFontSize: boolean;
  isAtMaxFontSize: boolean;
}>();

const emit = defineEmits<{
  retry: [];
  scroll: [];
  readerReady: [element: HTMLDivElement | null];
  previousSection: [];
  nextSection: [];
  decreaseFontSize: [];
  increaseFontSize: [];
  openFontModal: [];
  openChapterModal: [];
}>();

interface ActivePointer {
  id: number;
  startX: number;
  startY: number;
  startTime: number;
  maxMovement: number;
  cancelled: boolean;
}

const { t } = useI18n();
const { writesEnabled } = useWriteAccess();
const pageRef = ref<HTMLElement | null>(null);
const chromeVisible = ref(false);
const showGestureHint = ref(false);
const boundaryMessage = ref('');
let activePointer: ActivePointer | null = null;
let hintTimer: ReturnType<typeof setTimeout> | undefined;
let boundaryTimer: ReturnType<typeof setTimeout> | undefined;

function clearHint(): void {
  showGestureHint.value = false;
  if (hintTimer !== undefined) {
    clearTimeout(hintTimer);
    hintTimer = undefined;
  }
}

function showBoundary(message: string): void {
  boundaryMessage.value = message;
  if (boundaryTimer !== undefined) {
    clearTimeout(boundaryTimer);
  }
  boundaryTimer = setTimeout(() => {
    boundaryMessage.value = '';
    boundaryTimer = undefined;
  }, BOUNDARY_MESSAGE_DURATION_MS);
}

function onPointerDown(event: PointerEvent): void {
  // Android WebView emits both pointer and touch events for a physical touch.
  // Let TouchEvent own that path so one swipe cannot navigate twice; pointer
  // handling remains available for mouse and pen input.
  if (event.pointerType === 'touch') {
    return;
  }
  if (!event.isPrimary) {
    if (activePointer) activePointer.cancelled = true;
    return;
  }
  if ((event.pointerType === 'mouse' && event.button !== 0) || isReaderInteractiveTarget(event.target)) {
    return;
  }

  activePointer = {
    id: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    startTime: performance.now(),
    maxMovement: 0,
    cancelled: false
  };
}

function onPointerMove(event: PointerEvent): void {
  if (event.pointerType === 'touch') return;
  if (!activePointer || event.pointerId !== activePointer.id) return;
  const distance = Math.hypot(event.clientX - activePointer.startX, event.clientY - activePointer.startY);
  activePointer.maxMovement = Math.max(activePointer.maxMovement, distance);
}

function resetPointer(): void {
  activePointer = null;
}

function onPointerUp(event: PointerEvent): void {
  if (event.pointerType === 'touch') return;
  const pointer = activePointer;
  resetPointer();
  if (!pointer || pointer.id !== event.pointerId || pointer.cancelled) return;

  finishGesture(pointer, event.clientX, event.clientY);
}

function onPointerCancel(event: PointerEvent): void {
  if (event.pointerType === 'touch') return;
  resetPointer();
}

function findTouch(touches: TouchList, identifier: number): Touch | undefined {
  for (let index = 0; index < touches.length; index += 1) {
    const touch = touches.item(index);
    if (touch?.identifier === identifier) return touch;
  }
  return undefined;
}

function onTouchStart(event: TouchEvent): void {
  if (event.touches.length !== 1 || isReaderInteractiveTarget(event.target)) {
    resetPointer();
    return;
  }

  const touch = event.touches.item(0);
  if (!touch) return;
  activePointer = {
    id: touch.identifier,
    startX: touch.clientX,
    startY: touch.clientY,
    startTime: performance.now(),
    maxMovement: 0,
    cancelled: false
  };
}

function onTouchMove(event: TouchEvent): void {
  if (!activePointer) return;
  if (event.touches.length !== 1) {
    activePointer.cancelled = true;
    return;
  }

  const touch = findTouch(event.touches, activePointer.id);
  if (!touch) {
    activePointer.cancelled = true;
    return;
  }
  const distance = Math.hypot(touch.clientX - activePointer.startX, touch.clientY - activePointer.startY);
  activePointer.maxMovement = Math.max(activePointer.maxMovement, distance);
}

function onTouchEnd(event: TouchEvent): void {
  const pointer = activePointer;
  resetPointer();
  if (!pointer || pointer.cancelled || event.touches.length > 0) return;

  const touch = findTouch(event.changedTouches, pointer.id);
  if (!touch) return;
  finishGesture(pointer, touch.clientX, touch.clientY);
}

function finishGesture(pointer: ActivePointer, clientX: number, clientY: number): void {
  if (window.getSelection()?.isCollapsed === false) return;

  const deltaX = clientX - pointer.startX;
  const deltaY = clientY - pointer.startY;
  const gesture = classifyReaderGesture({
    deltaX,
    deltaY,
    maxMovement: Math.max(pointer.maxMovement, Math.hypot(deltaX, deltaY)),
    durationMs: performance.now() - pointer.startTime
  });

  if (gesture === 'next') {
    clearHint();
    if (props.currentSectionIndex >= props.sections.length - 1) {
      showBoundary(t('reader.mobile.lastSection'));
    } else {
      emit('nextSection');
    }
    return;
  }

  if (gesture === 'previous') {
    clearHint();
    if (props.currentSectionIndex <= 0) {
      showBoundary(t('reader.mobile.firstSection'));
    } else {
      emit('previousSection');
    }
    return;
  }

  if (
    gesture === 'tap' &&
    pageRef.value &&
    isInReaderCenter({ clientX, clientY }, pageRef.value.getBoundingClientRect())
  ) {
    clearHint();
    chromeVisible.value = !chromeVisible.value;
  }
}

onMounted(() => {
  if (window.localStorage.getItem(GESTURE_HINT_STORAGE_KEY) === '1') return;
  window.localStorage.setItem(GESTURE_HINT_STORAGE_KEY, '1');
  showGestureHint.value = true;
  hintTimer = setTimeout(clearHint, GESTURE_HINT_DURATION_MS);
});

watch(() => props.bookId, () => {
  chromeVisible.value = false;
  boundaryMessage.value = '';
  resetPointer();
});

onBeforeUnmount(() => {
  if (hintTimer !== undefined) clearTimeout(hintTimer);
  if (boundaryTimer !== undefined) clearTimeout(boundaryTimer);
});
</script>

<style scoped>
.mobile-reader-page {
  position: relative;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: linear-gradient(180deg, #fffaf0 0%, #fdf8ed 100%);
  color: #2f2a22;
  touch-action: pan-y pinch-zoom;
}

.mobile-reader-main,
.mobile-reader-document {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.mobile-reader-main,
.mobile-reader-document {
  display: flex;
  flex-direction: column;
}

.mobile-reader-page .mobile-reader-content {
  flex: 1;
  min-height: 0;
  max-height: none;
  padding:
    max(24px, env(safe-area-inset-top))
    max(20px, env(safe-area-inset-right))
    max(28px, env(safe-area-inset-bottom))
    max(20px, env(safe-area-inset-left));
  scrollbar-gutter: auto;
  overscroll-behavior-y: contain;
  touch-action: pan-y pinch-zoom;
}

.mobile-reader-page :deep(.reader-md-code) {
  touch-action: pan-x pan-y pinch-zoom;
}

.mobile-reader-status {
  margin: auto;
  max-width: min(84vw, 560px);
}

.mobile-reader-error {
  display: grid;
  gap: 10px;
}

.mobile-reader-error p {
  margin: 0;
}

.mobile-reader-error .button {
  justify-self: start;
}

.mobile-reader-warning {
  position: absolute;
  z-index: 20;
  top: max(12px, env(safe-area-inset-top));
  left: 16px;
  right: 16px;
  margin: 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(255, 226, 173, 0.94);
  color: #6f4c1f;
  font-size: 0.88rem;
}

.mobile-reader-topbar,
.mobile-reader-toolbar {
  position: absolute;
  z-index: 40;
  border: 1px solid rgba(122, 104, 72, 0.18);
  background: rgba(247, 242, 231, 0.96);
  box-shadow: 0 10px 28px rgba(73, 57, 34, 0.18);
  backdrop-filter: blur(12px);
}

.mobile-reader-topbar {
  top: calc(8px + env(safe-area-inset-top));
  left: max(10px, env(safe-area-inset-left));
  right: max(10px, env(safe-area-inset-right));
  min-height: 52px;
  display: grid;
  grid-template-columns: minmax(72px, 1fr) minmax(0, 2fr) minmax(44px, 1fr);
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  border-radius: 14px;
}

.mobile-reader-back {
  color: #5d513f;
  font-size: 0.86rem;
  text-decoration: none;
}

.mobile-reader-chapter {
  min-width: 0;
  display: grid;
  justify-items: center;
  gap: 2px;
  text-align: center;
}

.mobile-reader-chapter strong {
  max-width: 100%;
  overflow: hidden;
  color: #3f3529;
  font-size: 0.92rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-reader-chapter span,
.mobile-reader-progress {
  color: #6b5f4a;
  font-size: 0.76rem;
}

.mobile-reader-progress {
  justify-self: end;
}

.mobile-reader-toolbar {
  left: max(10px, env(safe-area-inset-left));
  right: max(10px, env(safe-area-inset-right));
  bottom: calc(10px + env(safe-area-inset-bottom));
  display: flex;
  align-items: center;
  justify-content: space-evenly;
  gap: 6px;
  padding: 8px;
  border-radius: 14px;
}

.mobile-reader-tool {
  display: flex;
  flex: 1 1 0;
  min-width: 0;
  height: 44px;
  min-height: 44px;
  padding: 0;
  border-radius: 10px;
  align-items: center;
  justify-content: center;
  color: #4b3f2f;
}

.mobile-reader-tool:disabled {
  opacity: 0.48;
}

.mobile-reader-font-tool {
  font-size: 0.92rem;
  font-weight: 650;
}

.mobile-reader-tool svg {
  width: 20px;
  height: 20px;
}

.mobile-reader-hint,
.mobile-reader-boundary {
  position: absolute;
  z-index: 50;
  left: 50%;
  max-width: calc(100% - 36px);
  margin: 0;
  padding: 9px 13px;
  border-radius: 999px;
  background: rgba(63, 53, 41, 0.88);
  color: #fffdf7;
  font-size: 0.84rem;
  text-align: center;
  transform: translateX(-50%);
  pointer-events: none;
}

.mobile-reader-hint {
  bottom: calc(28px + env(safe-area-inset-bottom));
}

.mobile-reader-boundary {
  top: 50%;
  transform: translate(-50%, -50%);
}

.mobile-reader-live {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  white-space: nowrap;
}

.reader-chrome-enter-active,
.reader-chrome-leave-active,
.reader-message-enter-active,
.reader-message-leave-active {
  transition: opacity 140ms ease, transform 140ms ease;
}

.reader-chrome-enter-from,
.reader-chrome-leave-to,
.reader-message-enter-from,
.reader-message-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .reader-chrome-enter-active,
  .reader-chrome-leave-active,
  .reader-message-enter-active,
  .reader-message-leave-active {
    transition: none;
  }
}
</style>
