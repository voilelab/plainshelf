<template>
  <TooltipProvider :delay-duration="300">
    <ToolbarRoot
      orientation="vertical"
      as="div"
      class="reader-side-actions"
      :class="{ 'reader-side-actions-writable': writesEnabled }"
      :aria-label="t('reader.actionsLabel')"
    >
      <TooltipRoot>
        <TooltipTrigger as-child>
          <ToolbarButton
            class="button reader-icon-button reader-font-button"
            :aria-label="t('reader.decreaseFontSize')"
            :disabled="isAtMinFontSize"
            @click="emit('decreaseFontSize')"
          >
            A−
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
            class="button reader-icon-button reader-font-button"
            :aria-label="t('reader.chooseFont')"
            @click="emit('openFontModal')"
          >
            Aa
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ t('reader.chooseFont') }}
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
            <Icon name="menu" />
          </ToolbarButton>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent class="reka-tooltip" :side-offset="6">
            {{ t('reader.showChapters') }}
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
import Icon from '@/components/Icon.vue';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useI18n } from '@/i18n';

defineProps<{
  isAtMinFontSize: boolean;
  isAtMaxFontSize: boolean;
  hasSections: boolean;
}>();

const emit = defineEmits<{
  decreaseFontSize: [];
  increaseFontSize: [];
  openFontModal: [];
  openChapterModal: [];
}>();

const { t } = useI18n();
const { writesEnabled } = useWriteAccess();
</script>

<style scoped src="../styles/reader-layout.css"></style>
<style scoped src="../styles/reader-content.css"></style>
