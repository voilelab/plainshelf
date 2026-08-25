<template>
  <section class="reader-page" data-reader-variant="desktop">
    <div class="reader-shell">
      <header class="reader-toolbar">
        <RouterLink :to="`/books/${bookId}`" class="reader-back">{{ t('reader.backToDetail') }}</RouterLink>
        <div class="reader-title">
          <span class="reader-kicker">{{ t('reader.title') }}</span>
          <h2>{{ title || bookId }}</h2>
        </div>
        <div class="reader-header-meta">
          <span class="reader-progress">{{ t('reader.progress', { percent: progressPercent }) }}</span>
        </div>
      </header>

      <div class="reader-layout">
        <ReaderSideActions
          :is-at-min-font-size="isAtMinFontSize"
          :is-at-max-font-size="isAtMaxFontSize"
          :has-sections="sections.length > 0"
          @decrease-font-size="emit('decreaseFontSize')"
          @increase-font-size="emit('increaseFontSize')"
          @open-font-modal="emit('openFontModal')"
          @open-chapter-modal="emit('openChapterModal')"
        />

        <main class="reader-main">
          <div v-if="loading" class="loading reader-status">{{ t('reader.loadingContent') }}</div>
          <div v-else-if="error" class="error reader-status reader-error" role="alert">
            <p>{{ error }}</p>
            <button class="button" type="button" @click="emit('retry')">{{ t('common.retry') }}</button>
          </div>

          <article v-else class="reader-document">
            <div v-if="sections.length > 0" class="reader-nav">
              <button
                class="button reader-nav-button"
                type="button"
                :disabled="currentSectionIndex <= 0"
                @click="emit('previousSection')"
              >
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
                @click="emit('nextSection')"
              >
                {{ t('common.next') }}
              </button>
            </div>

            <p v-if="saveError" class="reader-split-warning" role="status">
              {{ t('reader.autosaveFailed') }}
            </p>

            <ReaderContent
              :book-id="bookId"
              :source-id="sourceId"
              :book-format="bookFormat"
              :section="currentSection"
              @scroll="emit('scroll')"
              @ready="emit('readerReady', $event)"
            />
          </article>
        </main>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import ReaderContent from '@/features/reader/components/ReaderContent.vue';
import ReaderSideActions from '@/features/reader/components/ReaderSideActions.vue';
import type { ReaderSection } from '@/types/book';
import { useI18n } from '@/i18n';

defineProps<{
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

const { t } = useI18n();
</script>

<style scoped src="../styles/reader-layout.css"></style>
<style scoped src="../styles/reader-content.css"></style>
