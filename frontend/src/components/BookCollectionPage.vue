<template>
  <div>
    <div v-if="loading" class="loading">{{ shelfInitializing ? t('bookCollection.shelfInitializing') : t('bookCollection.loadingBooks') }}</div>
    <div v-else-if="shelfUnreachable" class="error collection-error" role="alert">
      <p>{{ t('bookCollection.shelfUnreachable') }}</p>
      <button type="button" class="button" @click="emit('retry')">{{ t('common.retry') }}</button>
    </div>
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

          <DropdownMenuRoot>
            <DropdownMenuTrigger class="button view-mode-trigger" type="button">
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
            </DropdownMenuTrigger>

            <DropdownMenuPortal>
              <DropdownMenuContent class="reka-menu view-mode-menu" align="end" :side-offset="6">
                <DropdownMenuRadioGroup :model-value="viewMode" @update:model-value="onViewModeSelect">
                  <DropdownMenuRadioItem
                    v-for="option in viewModeOptions"
                    :key="option.value"
                    class="reka-menu-item view-mode-option"
                    :value="option.value"
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
                  </DropdownMenuRadioItem>
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenuPortal>
          </DropdownMenuRoot>

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
import { computed, onMounted, ref, useSlots, watch } from 'vue';
import {
  DropdownMenuContent,
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  type AcceptableValue
} from 'reka-ui';
import BookCardView from './BookCardView.vue';
import BookListView from './BookListView.vue';
import BookTitleView from './BookTitleView.vue';
import Pagination from './Pagination.vue';
import type { Book } from '../types/book';
import {
  getStoredBooksViewMode,
  isBooksViewMode,
  setStoredBooksViewMode,
  type BooksViewMode
} from '../utils/booksViewMode';
import { useI18n } from '../i18n';

const props = withDefaults(defineProps<{
  title: string;
  books: Book[];
  loading?: boolean;
  shelfInitializing?: boolean;
  shelfUnreachable?: boolean;
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
  shelfInitializing: false,
  shelfUnreachable: false,
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

function onViewModeSelect(value: AcceptableValue): void {
  if (typeof value !== 'string' || !isBooksViewMode(value)) {
    return;
  }
  viewMode.value = value;
}

watch(viewMode, (mode) => {
  setStoredBooksViewMode(mode, props.viewModeStorageKey);
});

onMounted(() => {
  viewMode.value = getStoredBooksViewMode(props.viewModeStorageKey);
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

.view-mode-trigger {
  align-items: center;
  display: inline-flex;
  gap: 8px;
  min-width: 96px;
}

.view-mode-trigger-icon {
  color: var(--muted);
  display: inline-flex;
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

<!-- Unscoped: DropdownMenuContent/Item are portalled outside this
     component's DOM subtree, so this component's scope attribute never
     lands on them. Style portalled popper content with plain CSS. -->
<style>
.view-mode-svg {
  width: 14px;
  height: 14px;
}

.view-mode-menu {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 132px;
}

.view-mode-option {
  align-items: center;
  display: flex;
  gap: 8px;
}

.view-mode-option-icon {
  color: var(--muted);
  display: inline-flex;
}

.view-mode-option[data-state="checked"] {
  background: #f4f7fb;
  color: color-mix(in srgb, var(--text) 88%, var(--accent));
}
</style>
