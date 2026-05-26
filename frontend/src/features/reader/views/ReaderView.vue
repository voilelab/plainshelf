<template>
  <section class="reader-page" :style="readerStyleVars">
    <div class="reader-shell">
      <header class="reader-toolbar">
        <RouterLink :to="`/books/${id}`" class="reader-back">{{ t('reader.backToDetail') }}</RouterLink>
        <div class="reader-title">
          <span class="reader-kicker">{{ t('reader.title') }}</span>
          <h2>{{ title || id }}</h2>
        </div>
        <div class="reader-header-meta">
          <span class="reader-progress">{{ t('reader.progress', { percent: progress?.percent ?? 0 }) }}</span>
        </div>
      </header>

      <div class="reader-layout">
        <ReaderSideActions
          :is-at-min-font-size="isAtMinFontSize"
          :is-at-max-font-size="isAtMaxFontSize"
          :has-sections="sections.length > 0"
          :bookmarking="bookmarking"
          @decrease-font-size="decreaseFontSize"
          @increase-font-size="increaseFontSize"
          @open-chapter-modal="openChapterModal"
          @open-split-modal="openSplitModal"
          @bookmark-current="bookmarkCurrent"
        />

        <main class="reader-main">
          <div v-if="loading" class="loading reader-status">{{ t('reader.loadingContent') }}</div>
          <div v-else-if="error" class="error reader-status reader-error" role="alert">
            <p>{{ error }}</p>
            <button class="button" type="button" @click="fetchReaderData">{{ t('common.retry') }}</button>
          </div>

          <article v-else class="reader-document">
            <div class="reader-nav" v-if="sections.length > 0">
              <button class="button reader-nav-button" type="button" :disabled="currentSectionIndex <= 0" @click="goPrevSection">
                {{ t('common.prev') }}
              </button>
              <div class="reader-nav-center">
                <strong>{{ currentSectionIndex + 1 }} / {{ sections.length }}</strong>
                <span class="reader-nav-title">{{ currentSection?.title }}</span>
              </div>
              <button
                class="button reader-nav-button"
                type="button"
                :disabled="currentSectionIndex >= sections.length - 1"
                @click="goNextSection"
              >
                {{ t('common.next') }}
              </button>
            </div>

            <p v-if="splitWarning" class="reader-split-warning" role="status">{{ splitWarning }}</p>

            <div class="reader-content" @scroll="onScroll" ref="readerRef">
              <h3 v-if="currentSection?.title" class="reader-chapter-title">{{ currentSection.title }}</h3>
              <div class="reader-text">
                <template v-if="sectionBlocks.length > 0">
                  <component
                    :is="block.type === 'quote' ? 'blockquote' : 'p'"
                    v-for="(block, index) in sectionBlocks"
                    :key="`${currentSection?.index ?? 0}-${index}`"
                    class="reader-text-block"
                    :class="{ 'reader-text-quote': block.type === 'quote' }"
                  >
                    {{ block.text }}
                  </component>
                </template>
                <p v-else class="reader-text-block">{{ currentSection?.text ?? '' }}</p>
              </div>
            </div>
          </article>
        </main>
      </div>
    </div>

    <SplitConfigModal
      :open="isSplitModalOpen"
      :split-config="splitConfig"
      @close="closeSplitModal"
      @saved="handleSplitConfigSaved"
    />

    <ChapterModal
      :open="isChapterModalOpen"
      :sections="sections"
      :current-section-index="currentSectionIndex"
      @close="closeChapterModal"
      @select-section="selectSectionFromChapterModal"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import ChapterModal from '../components/ChapterModal.vue';
import ReaderSideActions from '../components/ReaderSideActions.vue';
import SplitConfigModal from '../components/SplitConfigModal.vue';
import { useDocumentTitle } from '../../../composables/useDocumentTitle';
import { useReader } from '../composables/useReader';
import { useReaderSettings } from '../composables/useReaderSettings';
import { parseReaderBlocks } from '../utils/parseReaderBlocks';
import type { SplitConfig } from '../../../types/book';
import { useI18n } from '../../../i18n';

const route = useRoute();
const id = computed(() => String(route.params.id));
const {
  title,
  sections,
  currentSectionIndex,
  currentSection,
  splitWarning,
  progress,
  loading,
  bookmarking,
  error,
  readerRef,
  fetchReaderData,
  onScroll,
  goPrevSection,
  goNextSection,
  goToSection,
  splitConfig,
  applySplitConfig,
  bookmarkCurrent
} = useReader(() => id.value);

const isSplitModalOpen = ref(false);
const isChapterModalOpen = ref(false);
const { fontSize, isAtMinFontSize, isAtMaxFontSize, increaseFontSize, decreaseFontSize } = useReaderSettings();
const { t } = useI18n();

const readerStyleVars = computed(() => ({
  '--reader-font-size': `${fontSize.value}px`
}));

useDocumentTitle(() => [t('reader.title'), title.value, t('app.name')]);

const sectionBlocks = computed(() => parseReaderBlocks(currentSection.value?.text ?? ''));

function openSplitModal(): void {
  isSplitModalOpen.value = true;
}

function closeSplitModal(): void {
  isSplitModalOpen.value = false;
}

function onDocumentKeydown(event: KeyboardEvent): void {
  const hasOpenModal = isSplitModalOpen.value || isChapterModalOpen.value;

  // Handle Escape key to close open modal
  if (event.key === 'Escape') {
    if (isSplitModalOpen.value) {
      closeSplitModal();
    } else if (isChapterModalOpen.value) {
      closeChapterModal();
    }
    return;
  }

  // Don't handle reader shortcuts when modal is open
  if (hasOpenModal) {
    return;
  }

  // Handle left/right arrow keys to navigate sections
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    // Don't handle if focus is on input/textarea/select/button/contenteditable elements
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

    // Navigate to previous/next section if not at boundary
    if (event.key === 'ArrowLeft') {
      if (currentSectionIndex.value > 0) {
        goPrevSection();
      }
    } else if (event.key === 'ArrowRight') {
      if (currentSectionIndex.value < sections.value.length - 1) {
        goNextSection();
      }
    }
  }
}

async function handleSplitConfigSaved(config: SplitConfig): Promise<void> {
  try {
    await applySplitConfig(config);
    closeSplitModal();
  } catch (err) {
    console.error('Failed to update split config', err);
  }
}

function openChapterModal(): void {
  isChapterModalOpen.value = true;
}

function closeChapterModal(): void {
  isChapterModalOpen.value = false;
}

async function selectSectionFromChapterModal(index: number): Promise<void> {
  await goToSection(index);
  isChapterModalOpen.value = false;
}

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown);
});

watch(id, () => {
  void fetchReaderData();
}, { immediate: true });

watch([isSplitModalOpen, isChapterModalOpen], ([splitOpen, chapterOpen]) => {
  document.body.style.overflow = splitOpen || chapterOpen ? 'hidden' : '';
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
  document.body.style.overflow = '';
});
</script>

<style scoped src="../styles/reader-layout.css"></style>
<style scoped src="../styles/reader-content.css"></style>
<style scoped src="../styles/reader-modal.css"></style>
