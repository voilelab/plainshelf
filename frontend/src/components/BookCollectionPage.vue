<template>
  <div>
    <div v-if="loading" class="loading">{{ t('bookCollection.loadingBooks') }}</div>
    <div v-else-if="error" class="error collection-error" role="alert">
      <p>{{ error }}</p>
      <button type="button" class="button" @click="emit('retry')">{{ t('common.retry') }}</button>
    </div>

    <div v-else class="bookshelf-content">
      <header class="bookshelf-header">
        <div>
          <h2 class="bookshelf-title">{{ title }}</h2>
          <p v-if="hasMetaLine" class="bookshelf-meta">
            <slot name="title-meta">
              {{ filterDescription }}
            </slot>
          </p>
        </div>

        <div class="bookshelf-toolbar">
          <p v-if="resolvedTotalLabel" class="bookshelf-count">{{ resolvedTotalLabel }}</p>

          <div class="view-mode-selector" ref="viewModeMenuRef">
            <button
              class="button view-mode-trigger"
              type="button"
              :aria-expanded="isViewModeMenuOpen ? 'true' : 'false'"
              aria-haspopup="menu"
              @click="toggleViewModeMenu"
            >
              <span class="view-mode-trigger-icon" aria-hidden="true">
                <svg v-if="viewMode === 'list'" viewBox="0 0 16 16" class="view-mode-svg">
                  <path d="M2 3.5h2v2H2zM5.5 4h8v1h-8zM2 7h2v2H2zM5.5 7.5h8v1h-8zM2 10.5h2v2H2zM5.5 11h8v1h-8z" fill="currentColor" />
                </svg>
                <svg v-else-if="viewMode === 'card'" viewBox="0 0 16 16" class="view-mode-svg">
                  <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" fill="currentColor" />
                </svg>
                <svg v-else viewBox="0 0 16 16" class="view-mode-svg">
                  <path d="M2 4h12v1H2zM2 7.5h12v1H2zM2 11h12v1H2z" fill="currentColor" />
                </svg>
              </span>
              <span>{{ currentViewModeLabel }}</span>
            </button>

            <div v-if="isViewModeMenuOpen" class="view-mode-menu panel" role="menu">
              <button
                v-for="option in viewModeOptions"
                :key="option.value"
                type="button"
                class="view-mode-option"
                :class="{ active: option.value === viewMode }"
                role="menuitemradio"
                :aria-checked="option.value === viewMode ? 'true' : 'false'"
                @click="selectViewMode(option.value)"
              >
                <span class="view-mode-option-icon" aria-hidden="true">
                  <svg v-if="option.value === 'list'" viewBox="0 0 16 16" class="view-mode-svg">
                    <path d="M2 3.5h2v2H2zM5.5 4h8v1h-8zM2 7h2v2H2zM5.5 7.5h8v1h-8zM2 10.5h2v2H2zM5.5 11h8v1h-8z" fill="currentColor" />
                  </svg>
                  <svg v-else-if="option.value === 'card'" viewBox="0 0 16 16" class="view-mode-svg">
                    <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" fill="currentColor" />
                  </svg>
                  <svg v-else viewBox="0 0 16 16" class="view-mode-svg">
                    <path d="M2 4h12v1H2zM2 7.5h12v1H2zM2 11h12v1H2z" fill="currentColor" />
                  </svg>
                </span>
                <span>{{ option.label }}</span>
              </button>
            </div>
          </div>

          <slot name="toolbar" />
        </div>
      </header>

      <div v-if="books.length === 0" class="panel empty-state">
        {{ emptyMessage }}
      </div>

      <BookListView
        v-else-if="viewMode === 'list'"
        :books="books"
        :show-edit-action="showEditAction"
        @select="emit('select', $event)"
        @edit="emit('edit', $event)"
      />

      <BookCardView
        v-else-if="viewMode === 'card'"
        :books="books"
        :show-edit-action="showEditAction"
        @select="emit('select', $event)"
        @edit="emit('edit', $event)"
      />

      <BookTitleView
        v-else
        :books="books"
        @select="emit('select', $event)"
      />

      <Pagination
        :page="page"
        :page-size="pageSize"
        :total="total"
        :page-size-options="pageSizeOptions ?? []"
        @update:page="emit('update:page', $event)"
        @update:page-size="emit('update:pageSize', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useSlots, watch } from 'vue';
import BookCardView from './BookCardView.vue';
import BookListView from './BookListView.vue';
import BookTitleView from './BookTitleView.vue';
import Pagination from './Pagination.vue';
import type { Book } from '../types/book';
import {
  getStoredBooksViewMode,
  setStoredBooksViewMode,
  type BooksViewMode
} from '../utils/booksViewMode';
import { useI18n } from '../i18n';

const props = withDefaults(defineProps<{
  title: string;
  books: Book[];
  loading?: boolean;
  error?: string;
  page: number;
  pageSize: number;
  total: number;
  emptyMessage: string;
  totalLabel?: string;
  count?: number;
  filterDescription?: string;
  showEditAction?: boolean;
  viewModeStorageKey?: string;
  pageSizeOptions?: number[];
}>(), {
  loading: false,
  error: '',
  totalLabel: '',
  count: undefined,
  filterDescription: '',
  showEditAction: false,
  viewModeStorageKey: undefined,
  pageSizeOptions: undefined
});

const emit = defineEmits<{
  (event: 'retry'): void;
  (event: 'select', id: string): void;
  (event: 'edit', id: string): void;
  (event: 'update:page', page: number): void;
  (event: 'update:pageSize', size: number): void;
}>();

const { t } = useI18n();
const viewModeOptions = computed<Array<{ value: BooksViewMode; label: string }>>(() => [
  { value: 'list', label: t('bookCollection.viewMode.list') },
  { value: 'card', label: t('bookCollection.viewMode.card') },
  { value: 'title', label: t('bookCollection.viewMode.title') }
]);

const viewMode = ref<BooksViewMode>('list');
const isViewModeMenuOpen = ref(false);
const viewModeMenuRef = ref<HTMLElement | null>(null);
const slots = useSlots();

const hasMetaLine = computed(() => !!props.filterDescription || !!slots['title-meta']);

const resolvedTotalLabel = computed(() => {
  if (props.totalLabel) {
    return props.totalLabel;
  }
  if (typeof props.count === 'number') {
    return t('bookCollection.booksCount', { count: props.count });
  }
  return '';
});

const currentViewModeLabel = computed(() => {
  return viewModeOptions.value.find((option) => option.value === viewMode.value)?.label ?? t('bookCollection.viewMode.list');
});

function toggleViewModeMenu(): void {
  isViewModeMenuOpen.value = !isViewModeMenuOpen.value;
}

function selectViewMode(mode: BooksViewMode): void {
  viewMode.value = mode;
  isViewModeMenuOpen.value = false;
}

function onWindowPointerDown(event: MouseEvent): void {
  if (!isViewModeMenuOpen.value) {
    return;
  }

  const target = event.target;
  if (!(target instanceof Node)) {
    return;
  }

  if (viewModeMenuRef.value?.contains(target)) {
    return;
  }

  isViewModeMenuOpen.value = false;
}

watch(viewMode, (mode) => {
  setStoredBooksViewMode(mode, props.viewModeStorageKey);
});

onMounted(() => {
  viewMode.value = getStoredBooksViewMode(props.viewModeStorageKey);
  window.addEventListener('mousedown', onWindowPointerDown);
});

onBeforeUnmount(() => {
  window.removeEventListener('mousedown', onWindowPointerDown);
});
</script>

<style scoped>
.bookshelf-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.collection-error {
  display: grid;
  gap: 10px;
}

.collection-error p {
  margin: 0;
}

.collection-error .button {
  justify-self: start;
}

.bookshelf-header {
  align-items: center;
  border-bottom: 1px solid #e6ecf3;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  min-height: 40px;
  padding-bottom: 8px;
}

.bookshelf-toolbar {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
}

.bookshelf-title {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.02em;
  margin: 0;
}

.bookshelf-meta {
  align-items: center;
  color: var(--muted);
  display: flex;
  flex-wrap: wrap;
  font-size: 12px;
  gap: 6px;
  margin: 4px 0 0;
}

.bookshelf-count {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
  white-space: nowrap;
}

.view-mode-selector {
  position: relative;
}

.view-mode-trigger {
  align-items: center;
  display: inline-flex;
  gap: 8px;
  min-width: 96px;
}

.view-mode-trigger-icon,
.view-mode-option-icon {
  color: var(--muted);
  display: inline-flex;
}

.view-mode-svg {
  width: 14px;
  height: 14px;
}

.view-mode-menu {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 132px;
  padding: 6px;
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 20;
}

.view-mode-option {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 8px;
  color: inherit;
  cursor: pointer;
  display: flex;
  gap: 8px;
  padding: 8px 10px;
  text-align: left;
  width: 100%;
}

.view-mode-option:hover,
.view-mode-option.active {
  background: #f4f7fb;
}

.view-mode-option.active {
  color: color-mix(in srgb, var(--text) 88%, var(--accent));
}

.empty-state {
  color: var(--muted);
  padding: 14px;
}

@media (max-width: 760px) {
  .bookshelf-header {
    align-items: stretch;
    flex-direction: column;
  }

  .bookshelf-toolbar {
    justify-content: space-between;
    width: 100%;
  }

  .view-mode-trigger {
    min-width: 88px;
  }
}
</style>
