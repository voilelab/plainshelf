<template>
  <ComicReaderView
    v-if="bookFormat === 'images'"
    :key="`${id}:${currentSourceId}`"
    :book-id="id" :source-id="currentSourceId" :title="title"
    :pages="pages" :index="currentSectionIndex" :direction="pageDirection"
    :percent="progress?.percent ?? 0" :loading="loading" :error="error" :save-error="saveError"
    :show-back-navigation="showBackNavigation"
    @back="goBack" @retry="loadBookAndApplySectionDeepLink"
    @previous="goPrevSection" @next="goNextSection" @select="goToSection"
  />
  <DesktopReaderView
    v-else-if="!isMobileReader"
    class="reading-surface"
    :data-reader-theme="theme"
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
    :show-back-navigation="showBackNavigation"
    :is-at-min-font-size="isAtMinFontSize"
    :is-at-max-font-size="isAtMaxFontSize"
    @retry="loadBookAndApplySectionDeepLink"
    @scroll="onScroll"
    @reader-ready="handleReaderReady"
    @back="goBack"
    @previous-section="goPrevSection"
    @next-section="goNextSection"
    @decrease-font-size="decreaseFontSize"
    @increase-font-size="increaseFontSize"
    @open-font-modal="openFontModal"
    @open-chapter-modal="openChapterModal"
  />

  <MobileReaderView
    v-else
    class="reading-surface"
    :data-reader-theme="theme"
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
    :show-back-navigation="showBackNavigation"
    :is-at-min-font-size="isAtMinFontSize"
    :is-at-max-font-size="isAtMaxFontSize"
    @retry="loadBookAndApplySectionDeepLink"
    @scroll="onScroll"
    @reader-ready="handleReaderReady"
    @back="goBack"
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
    :theme="theme"
    @close="closeChapterModal"
    @select-section="selectSectionFromChapterModal"
  />

  <FontSelectionModal
    :open="isFontModalOpen"
    :font-family="fontFamily"
    :theme="theme"
    @close="closeFontModal"
    @select="setFontFamily"
    @select-theme="setTheme"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { onBeforeRouteLeave, useRoute } from 'vue-router';
import ComicReaderView from '@/features/reader/components/ComicReaderView.vue';
import ChapterModal from '@/features/reader/components/ChapterModal.vue';
import DesktopReaderView from '@/features/reader/components/DesktopReaderView.vue';
import FontSelectionModal from '@/features/reader/components/FontSelectionModal.vue';
import MobileReaderView from '@/features/reader/components/MobileReaderView.vue';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useSafeBackNavigation } from '@/composables/useSafeBackNavigation';
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
import { isReaderRuntime } from '@/providers/runtime';

const route = useRoute();
const id = computed(() => String(route.params.id));
const { goBack } = useSafeBackNavigation(() => `/books/${id.value}`);
const showBackNavigation = !isReaderRuntime();
const isMobileReader = useMobileReaderPresentation();
const {
  title,
  bookFormat,
  pages,
  pageDirection,
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
  theme,
  isAtMinFontSize,
  isAtMaxFontSize,
  increaseFontSize,
  decreaseFontSize,
  setFontFamily,
  setTheme
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

function isTextEntryFocused(): boolean {
  const activeElement = document.activeElement;
  return (
    activeElement instanceof HTMLInputElement ||
    activeElement instanceof HTMLTextAreaElement ||
    activeElement instanceof HTMLSelectElement ||
    activeElement?.getAttribute?.('contenteditable') === 'true'
  );
}

function onDocumentKeydown(event: KeyboardEvent): void {
  if (isChapterModalOpen.value || isFontModalOpen.value || isTextEntryFocused()) {
    return;
  }
  if (event.key === 'Escape') {
    if (!showBackNavigation || event.defaultPrevented) {
      return;
    }
    // A Wails window on macOS leaves native fullscreen on an Escape the page does not handle.
    event.preventDefault();
    goBack();
    return;
  }
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
    return;
  }

  // macOS WKWebView/AppKit beeps on unhandled arrow keys, including chapter boundaries (d0c64e0c).
  event.preventDefault();

  const previousKey = bookFormat.value === 'images' && pageDirection.value === 'rtl' ? 'ArrowRight' : 'ArrowLeft';
  if (event.key === previousKey && currentSectionIndex.value > 0) {
    void goPrevSection();
  } else if (event.key !== previousKey && currentSectionIndex.value < sections.value.length - 1) {
    void goNextSection();
  }
}

async function loadBookAndApplySectionDeepLink(): Promise<void> {
  const requestedBookID = id.value;
  const requestedSection = parseSectionQuery(route.query.section);
  await fetchReaderData();

  if (requestedSection === null || id.value !== requestedBookID) {
    return;
  }

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
  void loadBookAndApplySectionDeepLink();
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
