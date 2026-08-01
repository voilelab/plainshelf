<template>
  <TooltipProvider :delay-duration="300">
    <ToolbarRoot orientation="vertical" as="div" class="reader-side-actions" :aria-label="t('reader.actionsLabel')">
      <TooltipRoot>
        <TooltipTrigger as-child>
          <ToolbarButton
            class="button reader-icon-button reader-font-button"
            :aria-label="t('reader.decreaseFontSize')"
            :disabled="isAtMinFontSize"
            @click="emit('decreaseFontSize')"
          >
            A-
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ t('reader.decreaseFontSize') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>

      <TooltipRoot>
        <TooltipTrigger as-child>
          <ToolbarButton
            class="button reader-icon-button reader-font-button"
            :aria-label="t('reader.increaseFontSize')"
            :disabled="isAtMaxFontSize"
            @click="emit('increaseFontSize')"
          >
            A+
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ t('reader.increaseFontSize') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>

      <TooltipRoot>
        <TooltipTrigger as-child>
          <ToolbarButton
            class="button reader-icon-button"
            :aria-label="t('reader.showChapters')"
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
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ t('reader.showChapters') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>

      <!-- Split settings rewrite the book's stored split config, so they are a
           write operation; the bookmark button below is not (mobile keeps
           reading progress on-device). -->
      <TooltipRoot v-if="writesEnabled">
        <TooltipTrigger as-child>
          <ToolbarButton
            class="button reader-icon-button"
            :aria-label="t('reader.splitSettings')"
            @click="emit('openSplitModal')"
          >
            <svg aria-hidden="true" viewBox="0 0 24 24" fill="none">
              <path d="M14 5l-9 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <path d="M10 5l9 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <circle cx="6" cy="5" r="2" stroke="currentColor" stroke-width="1.8" />
              <circle cx="18" cy="5" r="2" stroke="currentColor" stroke-width="1.8" />
            </svg>
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ t('reader.splitSettings') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>

      <TooltipRoot>
        <TooltipTrigger as-child>
          <ToolbarButton
            class="button reader-bookmark reader-icon-button"
            :aria-label="bookmarking ? t('reader.savingBookmark') : t('reader.saveBookmark')"
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
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ bookmarking ? t('reader.savingBookmark') : t('reader.saveBookmark') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>
    </ToolbarRoot>
  </TooltipProvider>
</template>

<script setup lang="ts">
import {
  ToolbarButton,
  ToolbarRoot,
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger
} from 'reka-ui';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useI18n } from '@/i18n';

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
const { writesEnabled } = useWriteAccess();
</script>

<style scoped src="../styles/reader-layout.css"></style>
<style scoped src="../styles/reader-content.css"></style>
