<template>
  <section class="reader-page" :style="readerStyleVars">
    <div class="reader-shell">
      <header class="reader-toolbar">
        <RouterLink :to="`/books/${id}`" class="reader-back">Back to detail</RouterLink>
        <div class="reader-title">
          <span class="reader-kicker">Reader</span>
          <h2>{{ title || id }}</h2>
        </div>
        <div class="reader-header-meta">
          <span class="reader-progress">Progress: {{ progress?.percent ?? 0 }}%</span>
        </div>
      </header>

      <div class="reader-layout">
        <aside class="reader-side-actions" aria-label="Reader actions">
          <button
            class="button reader-icon-button reader-font-button"
            type="button"
            aria-label="Decrease font size"
            title="Decrease font size"
            :disabled="isAtMinFontSize"
            @click="decreaseFontSize"
          >
            A-
          </button>
          <button
            class="button reader-icon-button reader-font-button"
            type="button"
            aria-label="Increase font size"
            title="Increase font size"
            :disabled="isAtMaxFontSize"
            @click="increaseFontSize"
          >
            A+
          </button>
          <button
            class="button reader-icon-button"
            type="button"
            aria-label="Show chapters"
            title="Show chapters"
            :disabled="sections.length === 0"
            @click="openChapterModal"
          >
            <svg aria-hidden="true" viewBox="0 0 24 24" fill="none">
              <path d="M8 6h12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <path d="M8 12h12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <path d="M8 18h12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <path d="M4 6h.01" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" />
              <path d="M4 12h.01" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" />
              <path d="M4 18h.01" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" />
            </svg>
          </button>
          <button
            class="button reader-icon-button"
            type="button"
            aria-label="Split settings"
            title="Split settings"
            @click="openSplitModal"
          >
            <svg aria-hidden="true" viewBox="0 0 24 24" fill="none">
              <path d="M14 5l-9 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <path d="M10 5l9 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <circle cx="6" cy="5" r="2" stroke="currentColor" stroke-width="1.8" />
              <circle cx="18" cy="5" r="2" stroke="currentColor" stroke-width="1.8" />
            </svg>
          </button>
          <button
            class="button reader-bookmark reader-icon-button"
            type="button"
            :aria-label="bookmarking ? 'Saving bookmark' : 'Save bookmark'"
            :title="bookmarking ? 'Saving bookmark' : 'Save bookmark'"
            :disabled="bookmarking"
            @click="bookmarkCurrent"
          >
            <svg aria-hidden="true" viewBox="0 0 24 24" fill="none">
              <path
                d="M7 5.5c0-.83.67-1.5 1.5-1.5h7c.83 0 1.5.67 1.5 1.5V20l-5-3-5 3V5.5z"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linejoin="round"
              />
            </svg>
          </button>
        </aside>

        <main class="reader-main">
          <div v-if="loading" class="loading reader-status">Loading content...</div>
          <div v-else-if="error" class="error reader-status reader-error" role="alert">
            <p>{{ error }}</p>
            <button class="button" type="button" @click="fetchReaderData">Retry</button>
          </div>

          <article v-else class="reader-document">
            <div class="reader-nav" v-if="sections.length > 0">
              <button class="button reader-nav-button" type="button" :disabled="currentSectionIndex <= 0" @click="goPrevSection">
                Prev
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
                Next
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

    <div v-if="isChapterModalOpen" class="chapter-modal-backdrop" role="presentation" @click="closeChapterModal">
      <section class="panel chapter-modal" role="dialog" aria-modal="true" aria-labelledby="chapter-modal-title" @click.stop>
        <header class="chapter-modal-header">
          <h3 id="chapter-modal-title">Chapters</h3>
          <button class="chapter-icon-close" type="button" aria-label="Close chapter dialog" @click="closeChapterModal">×</button>
        </header>

        <div class="chapter-modal-list">
          <button
            v-for="section in sections"
            :key="section.index"
            class="chapter-modal-item"
            :class="{ active: section.index === currentSectionIndex }"
            type="button"
            @click="selectSectionFromChapterModal(section.index)"
          >
            <span class="chapter-modal-item-index">{{ section.index + 1 }} / {{ sections.length }}</span>
            <span class="chapter-modal-item-title">{{ section.title }}</span>
          </button>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import SplitConfigModal from '../components/reader/modals/SplitConfigModal.vue';
import { useReader } from '../composables/useReader';
import { useReaderSettings } from '../composables/useReaderSettings';
import type { SplitConfig } from '../types/book';

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

type ReaderTextBlock = {
  type: 'paragraph' | 'quote';
  text: string;
};

const readerStyleVars = computed(() => ({
  '--reader-font-size': `${fontSize.value}px`
}));

const sectionBlocks = computed<ReaderTextBlock[]>(() => {
  const text = currentSection.value?.text ?? '';
  if (!text.trim()) {
    return [];
  }

  return text
    .split(/\n{2,}/)
    .map((chunk) => chunk.trim())
    .filter(Boolean)
    .map((chunk) => {
      const quoteLines = chunk
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean);
      const isQuote = quoteLines.length > 0 && quoteLines.every((line) => line.startsWith('>'));

      if (!isQuote) {
        return {
          type: 'paragraph' as const,
          text: chunk
        };
      }

      return {
        type: 'quote' as const,
        text: quoteLines.map((line) => line.replace(/^>\s?/, '')).join('\n')
      };
    });
});

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
  void fetchReaderData();
});

watch([isSplitModalOpen, isChapterModalOpen], ([splitOpen, chapterOpen]) => {
  document.body.style.overflow = splitOpen || chapterOpen ? 'hidden' : '';
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
  document.body.style.overflow = '';
});
</script>

<style scoped src="../styles/reader/reader-layout.css"></style>
<style scoped src="../styles/reader/reader-content.css"></style>
<style scoped src="../styles/reader/reader-modal.css"></style>
