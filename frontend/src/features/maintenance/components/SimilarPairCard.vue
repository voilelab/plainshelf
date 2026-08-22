<template>
  <article class="similar-pair-card">
    <DeleteModal
      :open="pendingDeleteId !== null"
      :item-name="pendingBook?.title || t('maintenance.duplicates.untitledBook')"
      :description="deleteDescription"
      :busy="deleting"
      :error="deleteError"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    />

    <header class="similar-pair-head">
      <p class="similar-pair-heading">
        <span class="similar-pair-score">{{ t('maintenance.similar.pairSimilarity', { percent }) }}</span>
        <span class="similar-pair-dot" aria-hidden="true">·</span>
        <span class="similar-pair-relation">{{ relationLabel }}</span>
      </p>
      <p class="similar-pair-desc">{{ relationDescription }}</p>
    </header>

    <div class="similar-pair-rows">
      <SimilarBookRow
        v-for="entry in orderedEntries"
        :key="entry.id"
        :book="entry.book"
        :char-count="entry.charCount"
        :source-count="entry.sourceCount"
        :is-more-complete="entry.isMoreComplete"
        :fewer-note="entry.fewerNote"
        :deleting="deleting && pendingDeleteId === entry.id"
        :disabled="deleting && pendingDeleteId !== entry.id"
        @open="openBook(entry.id)"
        @request-delete="requestDelete(entry.id)"
      />
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import type { SimilarBookPair } from '@/api/books';
import DeleteModal from '@/components/DeleteModal.vue';
import SimilarBookRow from '@/features/maintenance/components/SimilarBookRow.vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book } from '@/types/book';
import { useI18n } from '@/i18n';
import { estimatedDiffPer100, subsetShortfallPercent } from '@/utils/similarity';

const { t } = useI18n();

const props = defineProps<{
  pair: SimilarBookPair;
  /** Resolved from the shelf listing; undefined when it could not be matched. */
  bookA?: Book;
  bookB?: Book;
}>();

const emit = defineEmits<{
  deleted: [bookId: string];
}>();

const router = useRouter();

// Source counts are the one field neither the pair nor the book listing carries,
// so the card fetches them itself. null = unresolved or lookup failed; the row
// shows a dash rather than a wrong "0 sources".
const sourceCounts = ref<Record<string, number | null>>({});

const pendingDeleteId = ref<string | null>(null);
const deleting = ref(false);
const deleteError = ref('');

const percent = computed(() => Math.round(props.pair.jaccard * 100));

const relationLabel = computed(() => t(`maintenance.similar.relations.${props.pair.relation}`));

// The subset shortfall is measured from the normalized character counts the
// server already scored the pair with; the per-100 diff rate is the Mash
// estimate, which only reads true when differences are spread through the text —
// hence a distinct copy for the subset case (see the ticket's callout).
const shortfallPercent = computed(() =>
  subsetShortfallPercent(props.pair.norm_chars_a, props.pair.norm_chars_b)
);
const diffPer100 = computed(() => estimatedDiffPer100(props.pair.jaccard));

const relationDescription = computed(() => {
  const key = `maintenance.similar.card.relationDesc.${props.pair.relation}`;
  if (props.pair.relation === 'subset') {
    return t(key, { percent: shortfallPercent.value });
  }
  if (props.pair.relation === 'near_identical' || props.pair.relation === 'same_source') {
    return t(key, { count: diffPer100.value });
  }
  return t(key);
});

interface RowEntry {
  id: string;
  book: Book;
  charCount: number | null;
  sourceCount: number | null;
  isMoreComplete: boolean;
  fewerNote: string;
}

// A book that could not be matched in the listing still deserves a row so its
// copy is deletable; fall back to a title-only stand-in keyed by the pair id.
function bookFor(id: string, book: Book | undefined): Book {
  return book ?? { id, title: id, authors: [], tags: [], layers: [] };
}

function charCountOf(book: Book | undefined): number | null {
  return book?.char_count ?? null;
}

const orderedEntries = computed<RowEntry[]>(() => {
  const { pair } = props;
  const isSubset = pair.relation === 'subset';
  // The fuller side is the one with more normalized characters; for a subset it
  // is the superset to keep, and it leads so the trimmed copy reads as "below".
  const supersetIsA = pair.norm_chars_a >= pair.norm_chars_b;
  const moreCompleteId = supersetIsA ? pair.a : pair.b;

  const charA = charCountOf(props.bookA);
  const charB = charCountOf(props.bookB);
  let fewerId: string | null = null;
  if (charA !== null && charB !== null && charA !== charB) {
    fewerId = charA < charB ? pair.a : pair.b;
  }
  const delta = charA !== null && charB !== null ? Math.abs(charA - charB) : 0;
  const fewerNote = delta > 0 ? t('maintenance.similar.card.fewerChars', { count: delta.toLocaleString() }) : '';

  const build = (id: string, book: Book | undefined, charCount: number | null): RowEntry => ({
    id,
    book: bookFor(id, book),
    charCount,
    sourceCount: sourceCounts.value[id] ?? null,
    isMoreComplete: isSubset && id === moreCompleteId,
    fewerNote: id === fewerId ? fewerNote : ''
  });

  const entryA = build(pair.a, props.bookA, charA);
  const entryB = build(pair.b, props.bookB, charB);

  return isSubset && !supersetIsA ? [entryB, entryA] : [entryA, entryB];
});

const pendingBook = computed<Book | undefined>(() =>
  orderedEntries.value.find((entry) => entry.id === pendingDeleteId.value)?.book
);

const deleteDescription = computed(() => {
  const targetId = pendingDeleteId.value;
  if (targetId === null) {
    return '';
  }
  const target = orderedEntries.value.find((entry) => entry.id === targetId);
  const other = orderedEntries.value.find((entry) => entry.id !== targetId);
  if (!target || !other) {
    return '';
  }

  const parts: string[] = [];
  if (target.isMoreComplete) {
    parts.push(t('maintenance.similar.card.deleteMoreCompleteWarning'));
  }
  parts.push(t('maintenance.similar.card.deleteKeepNote', { otherTitle: other.book.title || other.id }));
  if (target.charCount !== null && other.charCount !== null) {
    parts.push(
      t('maintenance.similar.card.deleteCompare', {
        thisChars: target.charCount.toLocaleString(),
        otherChars: other.charCount.toLocaleString()
      })
    );
  }
  return parts.join(' ');
});

async function loadSourceCounts(): Promise<void> {
  const provider = getBookshelfProvider();
  const ids = [props.pair.a, props.pair.b];
  const results = await Promise.allSettled(ids.map((id) => provider.listSources(id)));
  const next: Record<string, number | null> = {};
  results.forEach((result, index) => {
    next[ids[index]] = result.status === 'fulfilled' ? result.value.length : null;
  });
  sourceCounts.value = next;
}

function openBook(id: string): void {
  void router.push(`/books/${id}`);
}

function requestDelete(id: string): void {
  if (deleting.value) {
    return;
  }
  deleteError.value = '';
  pendingDeleteId.value = id;
}

function cancelDelete(): void {
  if (deleting.value) {
    return;
  }
  pendingDeleteId.value = null;
  deleteError.value = '';
}

async function confirmDelete(): Promise<void> {
  const targetId = pendingDeleteId.value;
  if (targetId === null || deleting.value) {
    return;
  }

  deleting.value = true;
  deleteError.value = '';
  try {
    await bookshelfWriter().deleteBook(targetId);
    pendingDeleteId.value = null;
    emit('deleted', targetId);
  } catch (err) {
    deleteError.value = err instanceof Error ? err.message : t('maintenance.duplicates.deleteFailed');
  } finally {
    deleting.value = false;
  }
}

watch(
  () => [props.pair.a, props.pair.b],
  () => {
    sourceCounts.value = {};
    void loadSourceCounts();
  }
);

onMounted(() => {
  void loadSourceCounts();
});
</script>

<style scoped>
.similar-pair-card {
  border: 1px solid #e6ecf3;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

.similar-pair-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.similar-pair-heading {
  align-items: baseline;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 0;
}

.similar-pair-score {
  font-variant-numeric: tabular-nums;
  font-size: 15px;
  font-weight: 700;
}

.similar-pair-dot {
  color: var(--muted);
}

.similar-pair-relation {
  background: var(--accent-soft, #e6f0ff);
  border-radius: 999px;
  color: var(--accent, #2563eb);
  font-size: 12px;
  font-weight: 600;
  padding: 2px 10px;
}

.similar-pair-desc {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.4;
  margin: 0;
}

.similar-pair-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
