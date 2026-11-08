<template>
  <section class="duplicate-page">
    <header class="duplicate-page-header">
      <h2 class="duplicate-page-title">{{ t('maintenance.duplicateContent') }}</h2>
      <p class="duplicate-page-subtitle">{{ t('maintenance.duplicates.description') }}</p>
    </header>

    <div v-if="comicTotal > 0" class="toolbar-bar duplicate-comic-bar">
      <span class="toolbar-label">{{ comicNote }}</span>
      <span v-if="readOnly" class="toolbar-label duplicate-comic-muted">
        {{ t('maintenance.duplicates.comicHash.readOnly') }}
      </span>
      <template v-else>
        <button
          type="button"
          class="toolbar-control toolbar-button toolbar-regular"
          :disabled="hashRunning"
          @click="onHashComicPages"
        >
          {{ hashLabel }}
        </button>
        <span v-if="hashError" class="toolbar-label duplicate-comic-error" role="alert">{{ hashError }}</span>
      </template>
    </div>

    <div v-if="loading" class="loading">{{ t('maintenance.duplicates.scanning') }}</div>

    <div v-else-if="error" class="error duplicate-error" role="alert">
      <p>{{ error }}</p>
      <button type="button" class="button" @click="loadDuplicates">{{ t('common.retry') }}</button>
    </div>

    <div v-else-if="groups.length === 0 && overlaps.length === 0" class="duplicate-empty">
      <div class="duplicate-empty-icon" aria-hidden="true">✨</div>
      <p class="duplicate-empty-title">{{ t('maintenance.duplicates.empty') }}</p>
      <p class="duplicate-empty-subtitle">{{ t('maintenance.duplicates.emptyHint') }}</p>
    </div>

    <div v-else class="duplicate-groups">
      <DuplicateGroupCard
        v-for="(group, index) in groups"
        :key="`group-${index}`"
        :title="t('maintenance.duplicates.groupTitle', { index: index + 1 })"
        :summary="t('maintenance.duplicates.groupSummary', { count: group.length })"
        :books="group"
        @deleted="loadDuplicates"
      />
      <DuplicateGroupCard
        v-for="(overlap, index) in overlaps"
        :key="`overlap-${index}`"
        :title="t('maintenance.duplicates.overlapTitle', { index: index + 1 })"
        :summary="overlapSummary(overlap)"
        :books="[overlap.a, overlap.b]"
        @deleted="loadDuplicates"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import DuplicateGroupCard from '@/features/maintenance/components/DuplicateGroupCard.vue';
import type { ComicOverlapReport } from '@/api/books';
import type { Book } from '@/types/book';
import { useI18n } from '@/i18n';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useShelfCapabilities } from '@/composables/useShelfCapabilities';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useWriteAccess } from '@/composables/useWriteAccess';
import '@/styles/toolbar-controls.css';

const { t } = useI18n();

useDocumentTitle(() => [t('maintenance.duplicateContent')]);

type OverlapView = {
  a: Book;
  b: Book;
  shared: number;
  extraA: number;
  extraB: number;
};

const { writesEnabled } = useWriteAccess();
const readOnly = computed(() => !writesEnabled.value);

const loading = ref(false);
const error = ref('');
const groups = ref<Book[][]>([]);
const overlaps = ref<OverlapView[]>([]);
const comicReport = ref<ComicOverlapReport | null>(null);

const comicTotal = computed(() => comicReport.value?.total ?? 0);
const comicNote = computed(() => {
  const report = comicReport.value;
  if (!report) {
    return '';
  }
  return report.missing > 0
    ? t('maintenance.duplicates.comicHash.missingNote', { missing: report.missing, total: report.total })
    : t('maintenance.duplicates.comicHash.allHashed', { total: report.total });
});

function overlapSummary(overlap: OverlapView): string {
  return t('maintenance.duplicates.overlapSummary', {
    shared: overlap.shared,
    a: overlap.a.title || overlap.a.id,
    b: overlap.b.title || overlap.b.id,
    extraA: overlap.extraA,
    extraB: overlap.extraB
  });
}

async function loadDuplicates(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    const provider = getBookshelfProvider();
    const [duplicateGroups, report] = await Promise.all([
      provider.getDuplicateBookGroups(),
      useShelfCapabilities().fingerprint.value ? provider.getComicOverlaps() : Promise.resolve(null)
    ]);

    const books = new Map<string, Promise<Book>>();
    const bookOf = (id: string): Promise<Book> => {
      let book = books.get(id);
      if (!book) {
        book = provider.getBook(id);
        books.set(id, book);
      }
      return book;
    };

    const [resolvedGroups, resolvedOverlaps] = await Promise.all([
      Promise.all(duplicateGroups.map(async (ids) => await Promise.all(ids.map(bookOf)))),
      Promise.all(
        (report?.overlaps ?? []).map(async (overlap) => {
          const [a, b] = await Promise.all([bookOf(overlap.a), bookOf(overlap.b)]);
          return {
            a,
            b,
            shared: overlap.shared,
            extraA: overlap.pages_a - overlap.shared,
            extraB: overlap.pages_b - overlap.shared
          } satisfies OverlapView;
        })
      )
    ]);

    comicReport.value = report;
    groups.value = resolvedGroups;
    overlaps.value = resolvedOverlaps;
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('maintenance.duplicates.loadFailed');
    groups.value = [];
    overlaps.value = [];
  } finally {
    loading.value = false;
  }
}

const {
  percentage: hashPercentage,
  error: hashError,
  running: hashRunning,
  start: startHash
} = useTaskChainProgress({
  onSettled: () => loadDuplicates(),
  startFailedMessage: () => t('maintenance.duplicates.comicHash.failed'),
  pollFailedMessage: () => t('maintenance.duplicates.comicHash.failed')
});

const hashLabel = computed(() =>
  hashRunning.value
    ? t('maintenance.duplicates.comicHash.building', { percent: Math.round(hashPercentage.value) })
    : t('maintenance.duplicates.comicHash.build')
);

async function onHashComicPages(): Promise<void> {
  if (readOnly.value || hashRunning.value) {
    return;
  }
  await startHash(() => bookshelfWriter().startHashComicPages());
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
  border-bottom: 1px solid var(--border);
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

.duplicate-comic-bar {
  flex-wrap: wrap;
  margin-bottom: 10px;
}

.duplicate-comic-muted {
  color: var(--muted, #888);
}

.duplicate-comic-error {
  color: var(--danger, #c0392b);
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
