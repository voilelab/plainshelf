<template>
  <section class="duplicate-page">
    <header class="duplicate-page-header">
      <h2 class="duplicate-page-title">{{ t('maintenance.duplicateContent') }}</h2>
      <p class="duplicate-page-subtitle">{{ t('maintenance.duplicates.description') }}</p>
    </header>

    <div v-if="loading" class="loading">{{ t('maintenance.duplicates.scanning') }}</div>

    <div v-else-if="error" class="error duplicate-error" role="alert">
      <p>{{ error }}</p>
      <button type="button" class="button" @click="loadDuplicates">{{ t('common.retry') }}</button>
    </div>

    <div v-else-if="groups.length === 0" class="duplicate-empty">
      <div class="duplicate-empty-icon" aria-hidden="true">✨</div>
      <p class="duplicate-empty-title">{{ t('maintenance.duplicates.empty') }}</p>
      <p class="duplicate-empty-subtitle">{{ t('maintenance.duplicates.emptyHint') }}</p>
    </div>

    <div v-else class="duplicate-groups">
      <DuplicateGroupCard
        v-for="group in groups"
        :key="group.groupIndex"
        :group-index="group.groupIndex"
        :books="group.books"
        @deleted="loadDuplicates"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import DuplicateGroupCard from '@/features/maintenance/components/DuplicateGroupCard.vue';
import type { Book } from '@/types/book';
import { useI18n } from '@/i18n';
import { useDocumentTitle } from '@/composables/useDocumentTitle';

const { t } = useI18n();

// The other maintenance pages all set one; this was the odd one out.
useDocumentTitle(() => [t('maintenance.duplicateContent')]);

type DuplicateGroupView = {
  groupIndex: number;
  books: Book[];
};

const loading = ref(false);
const error = ref('');
const groups = ref<DuplicateGroupView[]>([]);

async function loadDuplicates(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    const provider = getBookshelfProvider();
    const duplicateGroups = await provider.getDuplicateBookGroups();

    const resolvedGroups = await Promise.all(
      duplicateGroups.map(async (groupIds, index) => {
        const books = await Promise.all(groupIds.map(async (bookId) => await provider.getBook(bookId)));

        return {
          groupIndex: index + 1,
          books
        } satisfies DuplicateGroupView;
      })
    );

    groups.value = resolvedGroups;
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('maintenance.duplicates.loadFailed');
    groups.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadDuplicates();
});
</script>

<style scoped>
.duplicate-page {
  margin: 0 auto;
  max-width: 900px;
  padding: 8px 0 24px;
  width: 100%;
}

.duplicate-page-header {
  border-bottom: 1px solid #e6ecf3;
  margin-bottom: 12px;
  padding-bottom: 8px;
}

.duplicate-page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.duplicate-page-subtitle {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.duplicate-error {
  display: grid;
  gap: 10px;
}

.duplicate-error p {
  margin: 0;
}

.duplicate-error .button {
  justify-self: start;
}

.duplicate-groups {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.duplicate-empty {
  align-items: center;
  display: flex;
  flex-direction: column;
  gap: 6px;
  justify-content: center;
  margin: 28px auto 8px;
  max-width: 360px;
  min-height: 150px;
  padding: 6px 0;
  text-align: center;
}

.duplicate-empty-icon {
  font-size: 30px;
  line-height: 1;
}

.duplicate-empty-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
}

.duplicate-empty-subtitle {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}
</style>
