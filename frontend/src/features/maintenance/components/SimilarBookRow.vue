<template>
  <article class="similar-book-row" :class="{ 'is-more-complete': isMoreComplete }">
    <img :src="coverSrc" :alt="book.title" class="similar-cover" @error="onCoverError" />

    <div class="similar-info">
      <div class="similar-title-line">
        <h4 class="similar-title">{{ book.title || t('maintenance.duplicates.untitledBook') }}</h4>
        <span v-if="isMoreComplete" class="similar-badge">{{ t('maintenance.similar.card.moreComplete') }}</span>
      </div>

      <dl class="similar-stats">
        <div class="similar-stat">
          <dt>{{ t('maintenance.similar.card.charsLabel') }}</dt>
          <dd>{{ charCountLabel }}</dd>
        </div>
        <div class="similar-stat">
          <dt>{{ t('maintenance.similar.card.formatLabel') }}</dt>
          <dd>{{ formatLabel }}</dd>
        </div>
        <div class="similar-stat">
          <dt>{{ t('maintenance.similar.card.sourcesLabel') }}</dt>
          <dd>{{ sourceCountLabel }}</dd>
        </div>
        <div class="similar-stat">
          <dt>{{ t('maintenance.similar.card.folderLabel') }}</dt>
          <dd>{{ layerLabel }}</dd>
        </div>
        <div v-if="addedLabel" class="similar-stat">
          <dt>{{ t('maintenance.similar.card.addedLabel') }}</dt>
          <dd>{{ addedLabel }}</dd>
        </div>
      </dl>

      <p v-if="fewerNote" class="similar-fewer">{{ fewerNote }}</p>
    </div>

    <div class="similar-actions">
      <button type="button" class="button similar-open" :disabled="disabled || deleting" @click="emit('open')">
        {{ t('maintenance.duplicates.open') }}
      </button>
      <button
        type="button"
        class="button danger similar-delete"
        :disabled="disabled || deleting"
        :aria-label="t('maintenance.duplicates.deleteLabel', { title: book.title || t('maintenance.duplicates.untitledBook') })"
        @click="emit('requestDelete')"
      >
        {{ deleting ? t('maintenance.duplicates.deleting') : t('maintenance.duplicates.delete') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import bookcover from '@/assets/bookcover.svg';
import { useI18n } from '@/i18n';
import type { Book } from '@/types/book';
import { formatDateLabel } from '@/utils/date';
import { getLayerPath, layerPathLabel } from '@/utils/layers';

const UNKNOWN = '—';

const { t } = useI18n();

const props = withDefaults(
  defineProps<{
    book: Book;
    /** From the shelf listing's char_count; null when the shelf could not report it. */
    charCount: number | null;
    /** Number of sources on record; null while unresolved or if the lookup failed. */
    sourceCount: number | null;
    /** Marks the fuller side of a `subset` pair — the one to keep. */
    isMoreComplete?: boolean;
    /** Pre-formatted "how much less" note (subset percentage or character delta). */
    fewerNote?: string;
    /** This row's own delete is in flight. */
    deleting?: boolean;
    /** Another row in the same card is mid-delete, so this one is inert. */
    disabled?: boolean;
  }>(),
  {
    isMoreComplete: false,
    fewerNote: '',
    deleting: false,
    disabled: false
  }
);

const emit = defineEmits<{
  open: [];
  requestDelete: [];
}>();

const hasCoverLoadError = ref(false);

const coverSrc = computed(() => {
  if (hasCoverLoadError.value) {
    return bookcover;
  }
  return props.book.cover_url || bookcover;
});

const charCountLabel = computed(() =>
  props.charCount === null ? UNKNOWN : props.charCount.toLocaleString()
);

const sourceCountLabel = computed(() =>
  props.sourceCount === null ? UNKNOWN : props.sourceCount.toLocaleString()
);

const formatLabel = computed(() => (props.book.format || 'txt').toUpperCase());

const layerLabel = computed(() => layerPathLabel(getLayerPath(props.book)));

const addedLabel = computed(() => formatDateLabel(props.book.created_at));

watch(
  () => props.book.cover_url,
  () => {
    hasCoverLoadError.value = false;
  }
);

function onCoverError(): void {
  hasCoverLoadError.value = true;
}
</script>

<style scoped>
.similar-book-row {
  align-items: start;
  border: 1px solid #e4ebf3;
  border-radius: 10px;
  display: grid;
  gap: 12px;
  grid-template-columns: auto minmax(0, 1fr) auto;
  padding: 10px 12px;
}

.similar-book-row.is-more-complete {
  border-color: var(--accent, #2563eb);
  background: var(--accent-soft, #f2f7ff);
}

.similar-cover {
  width: 42px;
  height: 62px;
  border-radius: 8px;
  border: 1px solid var(--border);
  object-fit: cover;
  background: #f4f7fb;
}

.similar-info {
  min-width: 0;
}

.similar-title-line {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.similar-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.3;
  overflow-wrap: anywhere;
}

.similar-badge {
  background: var(--accent, #2563eb);
  border-radius: 999px;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.02em;
  padding: 2px 8px;
  white-space: nowrap;
}

.similar-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin: 6px 0 0;
}

.similar-stat {
  display: flex;
  gap: 4px;
  font-size: 12px;
  line-height: 1.4;
}

.similar-stat dt {
  color: var(--muted);
  margin: 0;
}

.similar-stat dd {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  margin: 0;
  overflow-wrap: anywhere;
}

.similar-fewer {
  color: var(--muted);
  font-size: 12px;
  margin: 6px 0 0;
}

.similar-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.similar-open {
  font-size: 13px;
  font-weight: 600;
  padding: 6px 12px;
}

@media (max-width: 700px) {
  .similar-book-row {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .similar-actions {
    grid-column: 1 / -1;
    flex-direction: row;
    justify-self: end;
  }
}
</style>
