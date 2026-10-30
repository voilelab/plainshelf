<template>
  <section class="similar-page">
    <ConfirmModal
      :open="confirmForceOpen"
      :title="t('maintenance.similar.fingerprint.forceConfirmTitle')"
      :message="t('maintenance.similar.fingerprint.forceConfirmBody')"
      :confirm-text="t('maintenance.similar.fingerprint.forceConfirmAction')"
      @cancel="confirmForceOpen = false"
      @confirm="onConfirmForceRebuild"
    />

    <header class="similar-page-header">
      <h2 class="similar-page-title">{{ t('maintenance.similarContent') }}</h2>
      <p class="similar-page-subtitle">{{ t('maintenance.similar.description') }}</p>
    </header>

    <div v-if="showFingerprintBar" class="toolbar-bar similar-fingerprint-bar">
      <span class="toolbar-label">{{ fingerprintNote }}</span>
      <template v-if="readOnly">
        <span class="toolbar-label similar-fingerprint-readonly">
          {{ t('maintenance.similar.fingerprint.readOnly') }}
        </span>
      </template>
      <template v-else>
        <button
          v-if="missingCount > 0"
          type="button"
          class="toolbar-control toolbar-button toolbar-regular"
          :disabled="sweepBusy"
          @click="onBuildFingerprints"
        >
          {{ buildLabel }}
        </button>
        <button
          type="button"
          class="toolbar-control toolbar-button toolbar-small similar-fingerprint-force"
          :disabled="sweepBusy"
          @click="confirmForceOpen = true"
        >
          {{ forceLabel }}
        </button>
        <span v-if="buildError" class="toolbar-label similar-fingerprint-error" role="alert">
          {{ buildError }}
        </span>
        <span v-else-if="forceNote" class="toolbar-label similar-fingerprint-note" role="status">
          {{ forceNote }}
        </span>
      </template>
    </div>

    <SimilarityFilterBar
      v-model:tier="tier"
      v-model:advanced-open="advancedOpen"
      v-model:threshold="threshold"
      v-model:subset-only="subsetOnly"
    />

    <div v-if="loading" class="loading">{{ t('maintenance.similar.scanning') }}</div>

    <div v-else-if="estimate" class="similar-notice" role="status">
      <p class="similar-notice-title">{{ t('maintenance.similar.estimate.title') }}</p>
      <p>
        {{
          t('maintenance.similar.estimate.counts', {
            fingerprinted: estimate.fingerprinted.toLocaleString(),
            total: estimate.total.toLocaleString(),
            pairs: estimate.pairs.toLocaleString()
          })
        }}
      </p>
      <p>
        {{
          t('maintenance.similar.estimate.work', {
            work: estimate.work.toLocaleString(),
            seconds: estimate.seconds.toLocaleString()
          })
        }}
      </p>
      <button type="button" class="button" @click="onConfirmComparison">
        {{ t('maintenance.similar.estimate.confirm') }}
      </button>
    </div>

    <div v-else-if="error" class="error similar-error" role="alert">
      <p>{{ error }}</p>
      <button type="button" class="button" @click="load">{{ t('common.retry') }}</button>
    </div>

    <div v-else-if="visiblePairs.length === 0" class="similar-empty">
      <div class="similar-empty-icon" aria-hidden="true">✨</div>
      <p class="similar-empty-title">{{ t('maintenance.similar.empty') }}</p>
      <p class="similar-empty-subtitle">{{ t('maintenance.similar.emptyHint') }}</p>
    </div>

    <div v-else class="similar-results">
      <p class="similar-result-count">{{ t('maintenance.similar.resultCount', { count: visiblePairs.length }) }}</p>
      <div class="similar-pair-list">
        <SimilarPairCard
          v-for="pair in visiblePairs"
          :key="`${pair.a}:${pair.b}`"
          :pair="pair"
          :book-a="booksById.get(pair.a)"
          :book-b="booksById.get(pair.b)"
          :source-count-a="sourceCounts[pair.a] ?? null"
          :source-count-b="sourceCounts[pair.b] ?? null"
          :read-only="readOnly"
          @deleted="onDeleted"
        />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useShelfCapabilities } from '@/composables/useShelfCapabilities';
import { computed, onMounted, ref, watch } from 'vue';

import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import {
  FingerprintSweepBusyError,
  SimilarTooLargeError,
  type FingerprintStatus,
  type SimilarBookPair
} from '@/api/books';
import type { Book } from '@/types/book';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useI18n } from '@/i18n';
import SimilarityFilterBar from '@/features/maintenance/components/SimilarityFilterBar.vue';
import SimilarPairCard from '@/features/maintenance/components/SimilarPairCard.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import {
  DEFAULT_SIMILARITY_TIER,
  filterSimilarPairs,
  SIMILARITY_FLOOR,
  tierThreshold,
  type SimilarityTierKey
} from '@/utils/similarity';
import '@/styles/toolbar-controls.css';

const { t } = useI18n();

useDocumentTitle(() => [t('maintenance.similarContent')]);

const { writesEnabled } = useWriteAccess();
const readOnly = computed(() => !writesEnabled.value);

const loading = ref(false);
const error = ref('');
const estimate = ref<SimilarTooLargeError | null>(null);
const pairs = ref<SimilarBookPair[]>([]);
const fingerprint = ref<FingerprintStatus | null>(null);
const booksById = ref<Map<string, Book>>(new Map());
const sourceCounts = ref<Record<string, number | null>>({});
const sourceCountsInflight = new Set<string>();

const tier = ref<SimilarityTierKey>(DEFAULT_SIMILARITY_TIER);
const advancedOpen = ref(false);
const threshold = ref(tierThreshold(DEFAULT_SIMILARITY_TIER));
const subsetOnly = ref(false);

const activeThreshold = computed(() => (advancedOpen.value ? threshold.value : tierThreshold(tier.value)));

const visiblePairs = computed(() =>
  filterSimilarPairs(pairs.value, { threshold: activeThreshold.value, subsetOnly: subsetOnly.value })
);

const totalBooks = computed(() => fingerprint.value?.total ?? 0);
const missingCount = computed(() => fingerprint.value?.missing ?? 0);
const showFingerprintBar = computed(() => totalBooks.value > 0);

const fingerprintNote = computed(() =>
  missingCount.value > 0
    ? t('maintenance.similar.fingerprint.missingNote', { missing: missingCount.value, total: totalBooks.value })
    : t('maintenance.similar.fingerprint.allBuilt', { total: totalBooks.value })
);

async function ensureSourceCounts(ids: readonly string[]): Promise<void> {
  const provider = getBookshelfProvider();
  const todo = [...new Set(ids)].filter(
    (id) => !(id in sourceCounts.value) && !sourceCountsInflight.has(id)
  );
  if (todo.length === 0) {
    return;
  }

  todo.forEach((id) => sourceCountsInflight.add(id));
  const results = await Promise.allSettled(todo.map((id) => provider.listSources(id)));
  const next = { ...sourceCounts.value };
  results.forEach((result, index) => {
    next[todo[index]] = result.status === 'fulfilled' ? result.value.length : null;
  });
  sourceCounts.value = next;
  todo.forEach((id) => sourceCountsInflight.delete(id));
}

watch(
  visiblePairs,
  (shown) => {
    void ensureSourceCounts(shown.flatMap((pair) => [pair.a, pair.b]));
  },
  { immediate: true }
);

async function load(): Promise<void> {
  loading.value = true;
  error.value = '';
  estimate.value = null;
  sourceCounts.value = {};
  sourceCountsInflight.clear();

  const provider = getBookshelfProvider();
  const wantCharCount = useShelfCapabilities().charCount.value;
  const [statusResult, pairsResult, booksResult] = await Promise.allSettled([
    useShelfCapabilities().fingerprint.value ? provider.getFingerprintStatus() : Promise.resolve(null),
    useShelfCapabilities().fingerprint.value ? provider.getSimilarBookPairs(SIMILARITY_FLOOR) : Promise.resolve([]),
    provider.listBooks(1, Number.MAX_SAFE_INTEGER, wantCharCount ? { includeCharCount: true } : undefined)
  ]);

  fingerprint.value = statusResult.status === 'fulfilled' ? statusResult.value : null;

  booksById.value =
    booksResult.status === 'fulfilled'
      ? new Map(booksResult.value.items.map((book) => [book.id, book]))
      : new Map();

  if (pairsResult.status === 'fulfilled') {
    pairs.value = pairsResult.value;
  } else if (pairsResult.reason instanceof SimilarTooLargeError) {
    estimate.value = pairsResult.reason;
    pairs.value = [];
  } else {
    error.value =
      pairsResult.reason instanceof Error ? pairsResult.reason.message : t('maintenance.similar.loadFailed');
    pairs.value = [];
  }

  loading.value = false;
}

async function onConfirmComparison(): Promise<void> {
  if (!estimate.value || loading.value) {
    return;
  }

  loading.value = true;
  error.value = '';
  estimate.value = null;
  try {
    pairs.value = await getBookshelfProvider().getSimilarBookPairs(SIMILARITY_FLOOR, true);
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('maintenance.similar.loadFailed');
    pairs.value = [];
  } finally {
    loading.value = false;
  }
}

function onDeleted(bookId: string): void {
  pairs.value = pairs.value.filter((pair) => pair.a !== bookId && pair.b !== bookId);
  if (booksById.value.has(bookId)) {
    const next = new Map(booksById.value);
    next.delete(bookId);
    booksById.value = next;
  }
}

const runningAction = ref<'build' | 'force' | null>(null);

const forceStarting = ref(false);
const forceNote = ref('');

const confirmForceOpen = ref(false);

const {
  percentage: buildPercentage,
  error: buildError,
  running: buildRunning,
  start: startBuild
} = useTaskChainProgress({
  onSettled: () => {
    runningAction.value = null;
    return load();
  },
  startFailedMessage: () => t('maintenance.similar.fingerprint.failed'),
  pollFailedMessage: () => t('maintenance.similar.fingerprint.failed')
});

const sweepBusy = computed(() => buildRunning.value || forceStarting.value);

const buildLabel = computed(() =>
  buildRunning.value && runningAction.value === 'build'
    ? t('maintenance.similar.fingerprint.building', { percent: Math.round(buildPercentage.value) })
    : t('maintenance.similar.fingerprint.build')
);

const forceLabel = computed(() =>
  buildRunning.value && runningAction.value === 'force'
    ? t('maintenance.similar.fingerprint.forceRebuilding', { percent: Math.round(buildPercentage.value) })
    : t('maintenance.similar.fingerprint.forceRebuild')
);

async function onBuildFingerprints(): Promise<void> {
  if (readOnly.value || sweepBusy.value) {
    return;
  }
  forceNote.value = '';
  runningAction.value = 'build';
  await startBuild(() => bookshelfWriter().startFingerprintSources());
}

async function onConfirmForceRebuild(): Promise<void> {
  confirmForceOpen.value = false;
  await onForceRebuild();
}

async function onForceRebuild(): Promise<void> {
  if (readOnly.value || sweepBusy.value) {
    return;
  }
  forceNote.value = '';
  runningAction.value = 'force';
  forceStarting.value = true;

  let chainId: string;
  try {
    chainId = await bookshelfWriter().startFingerprintSources(true);
  } catch (err) {
    runningAction.value = null;
    forceNote.value =
      err instanceof FingerprintSweepBusyError
        ? t('maintenance.similar.fingerprint.busy')
        : t('maintenance.similar.fingerprint.failed');
    return;
  } finally {
    forceStarting.value = false;
  }

  await startBuild(() => Promise.resolve(chainId));
}

onMounted(() => {
  void load();
});
</script>

<style scoped>
.similar-page {
  margin: 0 auto;
  max-width: 900px;
  padding: 8px 0 24px;
  width: 100%;
}

.similar-page-header {
  border-bottom: 1px solid #e6ecf3;
  margin-bottom: 12px;
  padding-bottom: 8px;
}

.similar-page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.similar-page-subtitle {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.similar-fingerprint-bar {
  flex-wrap: wrap;
  margin-bottom: 10px;
}

.similar-fingerprint-readonly {
  color: var(--muted, #888);
}

/* Secondary next to the primary build button: a quieter, advanced action. */
.similar-fingerprint-force {
  color: var(--muted, #666);
}

.similar-fingerprint-force:not(:disabled):hover {
  color: inherit;
}

.similar-fingerprint-error {
  color: var(--danger, #c0392b);
}

/* Informational, not an error: a forced rebuild that found a sweep already
   running. */
.similar-fingerprint-note {
  color: var(--muted, #666);
}

.similar-error {
  display: grid;
  gap: 10px;
  margin-top: 12px;
}

.similar-error p {
  margin: 0;
}

.similar-error .button {
  justify-self: start;
}

.similar-notice {
  color: var(--muted);
  display: grid;
  gap: 8px;
  margin-top: 12px;
}

.similar-notice p {
  margin: 0;
}

.similar-notice-title {
  color: inherit;
  font-weight: 700;
}

.similar-notice .button {
  justify-self: start;
}

.similar-results {
  margin-top: 12px;
}

.similar-result-count {
  color: var(--muted);
  font-size: 13px;
  margin: 0 0 8px;
}

.similar-pair-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.similar-empty {
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

.similar-empty-icon {
  font-size: 30px;
  line-height: 1;
}

.similar-empty-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
}

.similar-empty-subtitle {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}
</style>
