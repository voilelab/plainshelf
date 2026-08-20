<template>
  <DesktopReaderView
    v-if="!isMobileReader"
    :style="readerStyleVars"
    :book-id="id"
    :title="title"
    :book-format="bookFormat"
    :source-id="currentSourceId"
    :sections="sections"
    :current-section-index="currentSectionIndex"
    :current-section="currentSection"
    :progress-percent="progress?.percent ?? 0"
    :loading="loading"
    :error="error"
    :save-error="saveError"
    :is-at-min-font-size="isAtMinFontSize"
    :is-at-max-font-size="isAtMaxFontSize"
    @retry="loadReader"
    @scroll="onScroll"
    @reader-ready="handleReaderReady"
    @previous-section="goPrevSection"
    @next-section="goNextSection"
    @decrease-font-size="decreaseFontSize"
    @increase-font-size="increaseFontSize"
    @open-font-modal="openFontModal"
    @open-chapter-modal="openChapterModal"
  />

  <MobileReaderView
    v-else
    :style="readerStyleVars"
    :book-id="id"
    :title="title"
    :book-format="bookFormat"
    :source-id="currentSourceId"
    :sections="sections"
    :current-section-index="currentSectionIndex"
    :current-section="currentSection"
    :progress-percent="progress?.percent ?? 0"
    :loading="loading"
    :error="error"
    :save-error="saveError"
    :is-at-min-font-size="isAtMinFontSize"
    :is-at-max-font-size="isAtMaxFontSize"
    @retry="loadReader"
    @scroll="onScroll"
    @reader-ready="handleReaderReady"
    @previous-section="goPrevSection"
    @next-section="goNextSection"
    @decrease-font-size="decreaseFontSize"
    @increase-font-size="increaseFontSize"
    @open-font-modal="openFontModal"
    @open-chapter-modal="openChapterModal"
  />

  <ChapterModal
    :open="isChapterModalOpen"
    :sections="sections"
    :current-section-index="currentSectionIndex"
    @close="closeChapterModal"
    @select-section="selectSectionFromChapterModal"
  />

  <FontSelectionModal
    :open="isFontModalOpen"
    :font-family="fontFamily"
    @close="closeFontModal"
    @select="setFontFamily"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { onBeforeRouteLeave, useRoute } from 'vue-router';
import ChapterModal from '@/features/reader/components/ChapterModal.vue';
import DesktopReaderView from '@/features/reader/components/DesktopReaderView.vue';
import FontSelectionModal from '@/features/reader/components/FontSelectionModal.vue';
import MobileReaderView from '@/features/reader/components/MobileReaderView.vue';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useReader } from '@/features/reader/composables/useReader';
import { useMobileReaderPresentation } from '@/features/reader/composables/useReaderPresentation';
import {
  DEFAULT_READER_FONT_SIZE,
  getReaderFontFamily,
  MOBILE_DEFAULT_FONT_SIZE,
  useReaderSettings
} from '@/features/reader/composables/useReaderSettings';
import { useReadingHeartbeat } from '@/features/reader/composables/useReadingHeartbeat';
import { parseSectionQuery } from '@/features/reader/utils/sectionDeepLink';
import { useI18n } from '@/i18n';

const route = useRoute();
const id = computed(() => String(route.params.id));
const isMobileReader = useMobileReaderPresentation();
const {
  title,
  bookFormat,
  currentSourceId,
  sections,
  currentSectionIndex,
  currentSection,
  progress,
  loading,
  error,
  saveError,
  readerRef,
  fetchReaderData,
  onScroll,
  goPrevSection,
  goNextSection,
  goToSection,
  syncCurrentScroll,
  flushReadingProgress,
  startProgressAutosave,
  stopProgressAutosave
} = useReader(() => id.value);

const isChapterModalOpen = ref(false);
const isFontModalOpen = ref(false);
const defaultFontSize = computed(() =>
  isMobileReader.value ? MOBILE_DEFAULT_FONT_SIZE : DEFAULT_READER_FONT_SIZE
);
const {
  fontSize,
  fontFamily,
  isAtMinFontSize,
  isAtMaxFontSize,
  increaseFontSize,
  decreaseFontSize,
  setFontFamily
} = useReaderSettings(defaultFontSize);
const { t } = useI18n();
const readingHeartbeat = useReadingHeartbeat(() => id.value);

const readerStyleVars = computed(() => ({
  '--reader-font-size': `${fontSize.value}px`,
  '--reader-font-family': getReaderFontFamily(fontFamily.value)
}));

useDocumentTitle(() => [t('reader.title'), title.value, t('app.name')]);

function handleReaderReady(element: HTMLDivElement | null): void {
  readerRef.value = element;
  if (element) {
    void syncCurrentScroll();
  }
}

function openFontModal(): void {
  isFontModalOpen.value = true;
}

function closeFontModal(): void {
  isFontModalOpen.value = false;
}

function openChapterModal(): void {
  isChapterModalOpen.value = true;
}

function closeChapterModal(): void {
  isChapterModalOpen.value = false;
}

function onDocumentKeydown(event: KeyboardEvent): void {
  if (isChapterModalOpen.value || isFontModalOpen.value) {
    return;
  }
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
    return;
  }

  const activeElement = document.activeElement;
  if (
    activeElement instanceof HTMLInputElement ||
    activeElement instanceof HTMLTextAreaElement ||
    activeElement instanceof HTMLSelectElement ||
    activeElement instanceof HTMLButtonElement ||
    activeElement?.getAttribute?.('contenteditable') === 'true'
  ) {
    return;
  }

  if (event.key === 'ArrowLeft' && currentSectionIndex.value > 0) {
    void goPrevSection();
  } else if (event.key === 'ArrowRight' && currentSectionIndex.value < sections.value.length - 1) {
    void goNextSection();
  }
}

/**
 * Loads the book, then honours a `?section=` link by opening that chapter
 * instead of the restored progress. Jumping writes progress, which is the
 * point: the reader asked for this chapter. The book ID is re-checked because
 * a navigation during the load leaves the newer book owning the reader.
 */
async function loadReader(): Promise<void> {
  const requestedBookID = id.value;
  const requestedSection = parseSectionQuery(route.query.section);
  await fetchReaderData();

  if (requestedSection === null || id.value !== requestedBookID) {
    return;
  }

  // goToSection clamps, so an out-of-range link opens the nearest chapter.
  await goToSection(requestedSection);
}

async function selectSectionFromChapterModal(index: number): Promise<void> {
  await goToSection(index);
  isChapterModalOpen.value = false;
}

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown);
  readingHeartbeat.start();
  startProgressAutosave();
});

onBeforeRouteLeave(async () => {
  await flushReadingProgress();
});

watch(id, () => {
  void loadReader();
}, { immediate: true });

watch([isChapterModalOpen, isFontModalOpen], ([chapterOpen, fontOpen]) => {
  document.body.style.overflow = chapterOpen || fontOpen ? 'hidden' : '';
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
  document.body.style.overflow = '';
  readingHeartbeat.stop();
  void stopProgressAutosave();
});
</script>
