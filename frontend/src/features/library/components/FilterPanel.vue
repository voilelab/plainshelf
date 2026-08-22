<template>
  <!-- Desktop: an anchored popover. -->
  <PopoverRoot v-if="!isMobileEnv" v-model:open="open">
    <PopoverTrigger :class="triggerClass" type="button" :aria-label="triggerAriaLabel">
      <span>{{ t('library.filters.button') }}</span>
      <span v-if="activeCount > 0" class="filter-badge">{{ activeCount }}</span>
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent class="filter-popover" align="end" :side-offset="6" :collision-padding="12">
        <header class="filter-popover-head">
          <h2 class="filter-popover-title">{{ t('library.filters.title') }}</h2>
          <PopoverClose class="filter-popover-close" :aria-label="t('library.filters.close')">✕</PopoverClose>
        </header>
        <FilterPanelBody
          :books="books"
          :read-only="readOnly"
          :char-count-supported="charCountSupported"
          :char-count-unknown-count="charCountUnknownCount"
          @stats-refreshed="emit('stats-refreshed')"
        />
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>

  <!-- Mobile: a bottom sheet. -->
  <template v-else>
    <button :class="triggerClass" type="button" :aria-label="triggerAriaLabel" @click="open = true">
      <span>{{ t('library.filters.button') }}</span>
      <span v-if="activeCount > 0" class="filter-badge">{{ activeCount }}</span>
    </button>
    <DialogRoot :open="open" @update:open="open = $event">
      <DialogPortal>
        <DialogOverlay class="filter-sheet-overlay" />
        <DialogContent class="filter-sheet" :aria-label="t('library.filters.title')">
          <header class="filter-popover-head">
            <DialogTitle class="filter-popover-title">{{ t('library.filters.title') }}</DialogTitle>
            <DialogClose class="filter-popover-close" :aria-label="t('library.filters.close')">✕</DialogClose>
          </header>
          <div class="filter-sheet-body">
            <FilterPanelBody
              :books="books"
              :read-only="readOnly"
              :char-count-supported="charCountSupported"
              :char-count-unknown-count="charCountUnknownCount"
              @stats-refreshed="emit('stats-refreshed')"
            />
          </div>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </template>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
  DialogClose,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  PopoverClose,
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger
} from 'reka-ui';
import type { Book } from '@/types/book';
import FilterPanelBody from './FilterPanelBody.vue';
import { useBookFilters } from '../composables/useBookFilters';
import { isMobileRuntime } from '@/providers/runtime';
import { useI18n } from '@/i18n';

defineProps<{
  books: Book[];
  readOnly: boolean;
  charCountSupported: boolean;
  charCountUnknownCount: number;
}>();

const emit = defineEmits<{
  (event: 'stats-refreshed'): void;
}>();

const { t } = useI18n();
const { activeCount } = useBookFilters();
const open = ref(false);
const isMobileEnv = computed(() => isMobileRuntime());

const triggerClass = 'button toolbar-control toolbar-button toolbar-regular filter-trigger';

const triggerAriaLabel = computed(() =>
  activeCount.value > 0
    ? t('library.filters.buttonActive', { count: activeCount.value })
    : t('library.filters.button')
);
</script>

<style scoped>
.filter-trigger {
  align-items: center;
  display: inline-flex;
  gap: 6px;
}

.filter-badge {
  align-items: center;
  background: var(--accent, #3b82f6);
  border-radius: 999px;
  color: #fff;
  display: inline-flex;
  font-size: 11px;
  font-weight: 600;
  justify-content: center;
  line-height: 1;
  min-width: 18px;
  padding: 2px 5px;
}
</style>

<!-- Unscoped: Popover/Dialog content is portalled out of this component's DOM
     subtree, so the scope attribute never lands on it. -->
<style>
.filter-popover {
  background: var(--bg, #fff);
  border: 1px solid var(--border, #d0d7e0);
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.18);
  max-height: min(70vh, 560px);
  overflow-y: auto;
  padding: 16px;
  width: min(340px, calc(100vw - 24px));
  z-index: var(--z-overlay, 20);
}

.filter-popover-head {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
}

.filter-popover-title {
  font-size: 15px;
  font-weight: 700;
  margin: 0;
}

.filter-popover-close {
  background: transparent;
  border: 0;
  color: var(--muted, #888);
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  padding: 4px;
}

.filter-popover-close:hover {
  color: var(--text, #333);
}

.filter-sheet-overlay {
  background: rgba(15, 23, 42, 0.38);
  inset: 0;
  position: fixed;
  z-index: var(--z-modal);
}

.filter-sheet {
  background: var(--bg, #fff);
  border-radius: 16px 16px 0 0;
  bottom: 0;
  box-shadow: 0 -8px 30px rgba(15, 23, 42, 0.22);
  left: 0;
  max-height: 80vh;
  overflow: hidden;
  padding: 16px 16px calc(16px + env(safe-area-inset-bottom, 0px));
  position: fixed;
  right: 0;
  z-index: var(--z-modal);
}

.filter-sheet-body {
  max-height: calc(80vh - 64px);
  overflow-y: auto;
}
</style>
