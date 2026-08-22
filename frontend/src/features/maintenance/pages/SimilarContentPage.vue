<template>
  <section class="similar-page">
    <header class="similar-page-header">
      <h2 class="similar-page-title">{{ t('maintenance.similarContent') }}</h2>
      <p class="similar-page-subtitle">{{ t('maintenance.similar.description') }}</p>
    </header>

    <!-- Fingerprint status bar: only shown when some books still lack a
         fingerprint, mirroring CharCountFilterBar's gap-and-build shape. -->
    <div v-if="showFingerprintBar" class="toolbar-bar similar-fingerprint-bar">
      <span class="toolbar-label">
        {{ t('maintenance.similar.fingerprint.missingNote', { missing: missingCount, total: totalBooks }) }}
      </span>
      <template v-if="readOnly">
        <span class="toolbar-label similar-fingerprint-readonly">
          {{ t('maintenance.similar.fingerprint.readOnly') }}
        </span>
      </template>
      <template v-else>
        <button
          type="button"
          class="toolbar-control toolbar-button toolbar-regular"
          :disabled="buildRunning"
          @click="onBuildFingerprints"
        >
          {{ buildLabel }}
        </button>
        <span v-if="buildError" class="toolbar-label similar-fingerprint-error" role="alert">
          {{ buildError }}
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

    <div v-else-if="tooLarge" class="similar-notice" role="status">
      <p>{{ t('maintenance.similar.tooLarge', { total: tooLarge.total, limit: tooLarge.limit }) }}</p>
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
      <!-- Skeleton rows: the pair card's own content is a separate ticket. -->
      <ul class="similar-pair-list">
        <li v-for="pair in visiblePairs" :key="`${pair.a}:${pair.b}`" class="similar-pair-row">
          <span class="similar-pair-relation">{{ relationLabel(pair.relation) }}</span>
          <span class="similar-pair-score">
            {{ t('maintenance.similar.pairSimilarity', { percent: Math.round(pair.jaccard * 100) }) }}
          </span>
          <code class="similar-pair-ids">{{ pair.a }} · {{ pair.b }}</code>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import { SimilarTooLargeError, type FingerprintStatus, type SimilarBookPair, type SimilarRelation } from '@/api/books';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useServerMode } from '@/composables/useServerMode';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useI18n } from '@/i18n';
import SimilarityFilterBar from '@/features/maintenance/components/SimilarityFilterBar.vue';
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

const { readOnly } = useServerMode();

const loading = ref(false);
const error = ref('');
const tooLarge = ref<{ total: number; limit: number } | null>(null);
const pairs = ref<SimilarBookPair[]>([]);
const fingerprint = ref<FingerprintStatus | null>(null);

// Filter state. The tier drives the threshold until the advanced slider is
// opened, which then owns it; the subset toggle is orthogonal to both.
const tier = ref<SimilarityTierKey>(DEFAULT_SIMILARITY_TIER);
const advancedOpen = ref(false);
const threshold = ref(tierThreshold(DEFAULT_SIMILARITY_TIER));
const subsetOnly = ref(false);

const activeThreshold = computed(() => (advancedOpen.value ? threshold.value : tierThreshold(tier.value)));

// Changing tier or toggle is a pure in-memory recompute over the one fetch.
const visiblePairs = computed(() =>
  filterSimilarPairs(pairs.value, { threshold: activeThreshold.value, subsetOnly: subsetOnly.value })
);

const totalBooks = computed(() => fingerprint.value?.total ?? 0);
const missingCount = computed(() => fingerprint.value?.missing ?? 0);
const showFingerprintBar = computed(() => missingCount.value > 0);

function relationLabel(relation: SimilarRelation): string {
  return t(`maintenance.similar.relations.${relation}`);
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = '';
  tooLarge.value = null;

  const provider = getBookshelfProvider();
  const [statusResult, pairsResult] = await Promise.allSettled([
    provider.getFingerprintStatus(),
    provider.getSimilarBookPairs(SIMILARITY_FLOOR)
  ]);

  // Coverage is best-effort: its failure must not hide an otherwise good list.
  fingerprint.value = statusResult.status === 'fulfilled' ? statusResult.value : null;

  if (pairsResult.status === 'fulfilled') {
    pairs.value = pairsResult.value;
  } else if (pairsResult.reason instanceof SimilarTooLargeError) {
    tooLarge.value = { total: pairsResult.reason.total, limit: pairsResult.reason.limit };
    pairs.value = [];
  } else {
    error.value =
      pairsResult.reason instanceof Error ? pairsResult.reason.message : t('maintenance.similar.loadFailed');
    pairs.value = [];
  }

  loading.value = false;
}

const {
  percentage: buildPercentage,
  error: buildError,
  running: buildRunning,
  start: startBuild
} = useTaskChainProgress({
  onSettled: () => load(),
  startFailedMessage: () => t('maintenance.similar.fingerprint.failed'),
  pollFailedMessage: () => t('maintenance.similar.fingerprint.failed')
});

const buildLabel = computed(() =>
  buildRunning.value
    ? t('maintenance.similar.fingerprint.building', { percent: Math.round(buildPercentage.value) })
    : t('maintenance.similar.fingerprint.build')
);

async function onBuildFingerprints(): Promise<void> {
  if (readOnly.value) {
    return;
  }
  await startBuild(() => bookshelfWriter().startFingerprintSources());
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

.similar-fingerprint-error {
  color: var(--danger, #c0392b);
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
  margin-top: 12px;
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
  gap: 8px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.similar-pair-row {
  align-items: center;
  border: 1px solid #e6ecf3;
  border-radius: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding: 10px 12px;
}

.similar-pair-relation {
  background: var(--accent-soft, #e6f0ff);
  border-radius: 999px;
  color: var(--accent, #2563eb);
  font-size: 12px;
  font-weight: 600;
  padding: 2px 10px;
}

.similar-pair-score {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.similar-pair-ids {
  color: var(--muted, #888);
  font-size: 12px;
  overflow-wrap: anywhere;
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
