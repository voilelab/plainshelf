<template>
  <aside class="reader-side-actions" :aria-label="t('reader.actionsLabel')">
    <button
      class="button reader-icon-button reader-font-button"
      type="button"
      :aria-label="t('reader.decreaseFontSize')"
      :title="t('reader.decreaseFontSize')"
      :disabled="isAtMinFontSize"
      @click="emit('decreaseFontSize')"
    >
      A-
    </button>
    <button
      class="button reader-icon-button reader-font-button"
      type="button"
      :aria-label="t('reader.increaseFontSize')"
      :title="t('reader.increaseFontSize')"
      :disabled="isAtMaxFontSize"
      @click="emit('increaseFontSize')"
    >
      A+
    </button>
    <button
      class="button reader-icon-button"
      type="button"
      :aria-label="t('reader.showChapters')"
      :title="t('reader.showChapters')"
      :disabled="!hasSections"
      @click="emit('openChapterModal')"
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
      :aria-label="t('reader.splitSettings')"
      :title="t('reader.splitSettings')"
      @click="emit('openSplitModal')"
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
      :aria-label="bookmarking ? t('reader.savingBookmark') : t('reader.saveBookmark')"
      :title="bookmarking ? t('reader.savingBookmark') : t('reader.saveBookmark')"
      :disabled="bookmarking"
      @click="emit('bookmarkCurrent')"
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
</template>

<script setup lang="ts">
import { useI18n } from '../../../i18n';

defineProps<{
  isAtMinFontSize: boolean;
  isAtMaxFontSize: boolean;
  hasSections: boolean;
  bookmarking: boolean;
}>();

const emit = defineEmits<{
  decreaseFontSize: [];
  increaseFontSize: [];
  openChapterModal: [];
  openSplitModal: [];
  bookmarkCurrent: [];
}>();

const { t } = useI18n();
</script>

<style scoped src="../styles/reader-layout.css"></style>
<style scoped src="../styles/reader-content.css"></style>
