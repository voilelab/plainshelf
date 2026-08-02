<template>
  <div>
    <DeleteModal
      :open="!!deleteTarget"
      :item-name="deleteTarget?.title || ''"
      :description="DELETE_BOOK_DESCRIPTION"
      :busy="deleting"
      :error="actionError"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    />
    <p v-if="actionError && !deleteTarget" class="error" role="alert">{{ actionError }}</p>
    <BookCollectionPage
      :title="heading"
      :books="visibleBooks"
      :loading="loading"
      :error="error"
      :page="page"
      :page-size="pageSize"
      :total="filteredBooks.length"
      :count="filteredBooks.length"
      :empty-message="emptyMessage"
      :filter-description="filterDescription"
      :show-edit-action="true"
      :can-open-book-folder="canOpenBookFolder"
      :read-only="readOnly"
      :page-size-options="PAGE_SIZE_OPTIONS"
      @retry="loadBooks"
      @activate="openDetail($event.id)"
      @edit="goEdit"
      @read="goRead"
      @open-book-folder="onOpenBookFolder"
      @download="onDownloadBook"
      @delete="onRequestDeleteBook"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    >
      <template v-if="thresholdConfig" #toolbar>
        <div class="toolbar-bar">
          <label class="toolbar-label" :for="THRESHOLD_INPUT_ID">{{ thresholdLabel }}</label>
          <input
            :id="THRESHOLD_INPUT_ID"
            class="toolbar-control toolbar-input threshold-input"
            type="number"
            inputmode="numeric"
            :min="thresholdConfig.min"
            :step="thresholdConfig.step"
            :value="threshold"
            @change="onThresholdChange"
          />
        </div>
      </template>
    </BookCollectionPage>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import BookCollectionPage from '@/components/BookCollectionPage.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import { DELETE_BOOK_DESCRIPTION } from '@/composables/useBookActions';
import { useBookCollectionActions } from '@/composables/useBookCollectionActions';
import { useBookCollectionRoute } from '@/composables/useBookCollectionRoute';
import { useBookStore } from '@/composables/useBookStore';
import { useCharCountBooks } from '@/features/maintenance/composables/useCharCountBooks';
import { toSingleQueryValue } from '@/composables/useBookPagination';
import {
  MAINTENANCE_BOOK_FILTERS,
  clampThreshold,
  hasUnknownCharCount,
  type MaintenanceBookFilter
} from '@/utils/maintenance';
import { useI18n } from '@/i18n';
import '@/styles/toolbar-controls.css';

const THRESHOLD_INPUT_ID = 'maintenance-threshold';

const props = defineProps<{
  filter: MaintenanceBookFilter;
}>();

const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const filterConfig = computed(() => MAINTENANCE_BOOK_FILTERS[props.filter]);

// Filters that read char_count need the opt-in listing; every other filter keeps
// using the shared store so it stays on the cheap /books request.
const bookStore = useBookStore();
const charCountBooks = useCharCountBooks();
const source = computed(() => (filterConfig.value.requiresCharCount ? charCountBooks : bookStore));

const books = computed(() => source.value.books.value);
const loading = computed(() => source.value.loading.value);
const error = computed(() => source.value.error.value);

const heading = computed(() => {
  return t(filterConfig.value.titleKey);
});

const emptyMessage = computed(() => {
  return t(filterConfig.value.emptyMessageKey);
});

const thresholdConfig = computed(() => filterConfig.value.threshold);

const threshold = computed(() => {
  const config = thresholdConfig.value;
  if (!config) {
    return 0;
  }

  return clampThreshold(toSingleQueryValue(route.query.maxChars), config);
});

const filteredBooks = computed(() => {
  return books.value.filter((book) => filterConfig.value.predicate(book, { threshold: threshold.value }));
});

const thresholdLabel = computed(() => {
  const config = thresholdConfig.value;
  return config ? t(config.labelKey) : '';
});

const filterDescription = computed(() => {
  const descriptionKey = filterConfig.value.filterDescriptionKey;
  if (!descriptionKey) {
    return '';
  }

  const description = t(descriptionKey, { threshold: threshold.value.toLocaleString() });
  if (!filterConfig.value.requiresCharCount) {
    return description;
  }

  const unknownCount = filteredBooks.value.filter(hasUnknownCharCount).length;
  if (unknownCount === 0) {
    return description;
  }

  return `${description} — ${t('maintenance.lowCharCount.unknownNote', { count: unknownCount })}`;
});

function buildQuery(overrides: { page?: number; maxChars?: number }): Record<string, string> {
  const nextQuery = {
    ...route.query
  } as Record<string, string>;

  if (overrides.page !== undefined) {
    delete nextQuery.page;
    nextQuery.page = String(overrides.page);
  }

  if (overrides.maxChars !== undefined) {
    delete nextQuery.maxChars;
    nextQuery.maxChars = String(overrides.maxChars);
  }

  return nextQuery;
}

const { page, pageSize, visibleBooks, onPageChange, onPageSizeChange, PAGE_SIZE_OPTIONS } =
  useBookCollectionRoute({
    items: filteredBooks,
    buildQuery: (nextPage) => buildQuery({ page: nextPage })
  });

// Committed on change (blur, Enter, spinner) rather than on every keystroke, so
// a partially typed number never filters the list or fills up history.
function onThresholdChange(event: Event): void {
  const config = thresholdConfig.value;
  if (!config) {
    return;
  }

  const nextThreshold = clampThreshold((event.target as HTMLInputElement).value, config);
  void router.replace({
    path: route.path,
    query: buildQuery({ page: 1, maxChars: nextThreshold })
  });
}

async function loadBooks(): Promise<void> {
  await source.value.fetchBooks();
}

const {
  canOpenBookFolder,
  actionError,
  deleteTarget,
  deleting,
  readOnly,
  goRead,
  openDetail,
  goEdit,
  cancelDelete,
  confirmDelete,
  onOpenBookFolder,
  onDownloadBook,
  onRequestDeleteBook
} = useBookCollectionActions({
  books,
  onDeleted: () => {
    void loadBooks();
  }
});

onMounted(() => {
  void loadBooks();
});
</script>

<style scoped>
.threshold-input {
  width: 96px;
}
</style>
