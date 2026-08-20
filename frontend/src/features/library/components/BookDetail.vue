<template>
  <div class="detail-main">
    <header class="detail-heading">
      <LayerBreadcrumb :layers="book.layers" />
      <h2 class="detail-title">{{ book.title }}</h2>
      <p v-if="book.authors.length > 0" class="detail-authors">{{ formatList(book.authors) }}</p>

      <div v-if="hasRating || book.tags.length > 0" class="summary-signals">
        <span
          v-if="hasRating"
          class="rating-text"
          :aria-label="t('bookDetail.ratingLabel', { rating: normalizedRating })"
        >
          <span aria-hidden="true">{{ formattedRating }}</span>
        </span>
        <ul v-if="book.tags.length > 0" class="tag-list" :aria-label="t('bookDetail.fields.tags')">
          <li v-for="tag in book.tags" :key="tag" class="tag-pill">{{ tag }}</li>
        </ul>
      </div>

      <dl v-if="quickFacts.length > 0" class="quick-facts">
        <div v-for="fact in quickFacts" :key="fact.label" class="quick-fact">
          <dt>{{ fact.label }}</dt>
          <dd>{{ fact.value }}</dd>
        </div>
      </dl>
    </header>

    <div class="reading-slot">
      <slot name="reading" />
    </div>

    <div class="detail-sections">
      <section v-if="chapters.length > 0" class="detail-card detail-card-chapters">
        <h3>{{ t('bookDetail.sections.chapters') }}</h3>
        <ul class="chapter-list">
          <li v-for="chapter in visibleChapters" :key="chapter.index">
            <button class="chapter-item" type="button" @click="emit('selectChapter', chapter.index)">
              <span class="chapter-item-index">{{ chapter.index + 1 }}</span>
              <span class="chapter-item-title">{{ chapter.title }}</span>
            </button>
          </li>
        </ul>
        <button
          v-if="chapters.length > CHAPTER_PREVIEW_LIMIT"
          class="chapter-toggle"
          type="button"
          @click="showAllChapters = !showAllChapters"
        >
          {{ showAllChapters
            ? t('bookDetail.chapters.showLess')
            : t('bookDetail.chapters.showAll', { count: chapters.length }) }}
        </button>
      </section>

      <section v-if="publicationRows.length > 0" class="detail-card">
        <h3>{{ t('bookDetail.sections.publication') }}</h3>
        <dl class="detail-definition-list">
          <div v-for="row in publicationRows" :key="row.label" class="detail-definition-row">
            <dt>{{ row.label }}</dt>
            <dd :class="row.className">{{ row.value }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="contentRows.length > 0" class="detail-card">
        <h3>{{ t('bookDetail.sections.content') }}</h3>
        <dl class="detail-definition-list">
          <div v-for="row in contentRows" :key="row.label" class="detail-definition-row">
            <dt>{{ row.label }}</dt>
            <dd>{{ row.value }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="noteRows.length > 0" class="detail-card detail-card-notes">
        <h3>{{ t('bookDetail.sections.notes') }}</h3>
        <dl class="detail-definition-list">
          <div v-for="row in noteRows" :key="row.label" class="detail-definition-row note-row">
            <dt>{{ row.label }}</dt>
            <dd>{{ row.value }}</dd>
          </div>
        </dl>
      </section>

      <p v-if="!hasDetailSections" class="detail-empty">{{ t('bookDetail.emptyDetails') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import LayerBreadcrumb from './LayerBreadcrumb.vue';
import type { Book, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { MarkdownChapterListItem } from '@/utils/markdownChapters';
import { formatLanguage } from '@/utils/language';
import { formatDateLabel } from '@/utils/date';
import { useI18n } from '@/i18n';

const props = withDefaults(
  defineProps<{
    book: Book;
    progress?: ReadingProgress | null;
    currentSource?: SourceMeta | null;
    chapters?: MarkdownChapterListItem[];
  }>(),
  { progress: null, currentSource: null, chapters: () => [] }
);

const emit = defineEmits<{
  selectChapter: [index: number];
}>();

const { t } = useI18n();

/** A book with hundreds of chapters must not push the rest of the page away. */
const CHAPTER_PREVIEW_LIMIT = 12;
const showAllChapters = ref(false);
const chapters = computed(() => props.chapters);
const visibleChapters = computed(() =>
  showAllChapters.value ? chapters.value : chapters.value.slice(0, CHAPTER_PREVIEW_LIMIT)
);

watch(() => props.book.id, () => {
  showAllChapters.value = false;
});

interface DetailRow {
  label: string;
  value: string;
  className?: string;
}

function formatList(values: string[]): string {
  return values.join(', ');
}

function formatNumber(value?: number): string {
  return new Intl.NumberFormat().format(value ?? 0);
}

const normalizedRating = computed(() => Math.min(5, Math.max(0, Math.trunc(props.book.star ?? 0))));
const hasRating = computed(() => typeof props.book.star === 'number' && Number.isFinite(props.book.star));
const formattedRating = computed(() => `${'★'.repeat(normalizedRating.value)}${'☆'.repeat(5 - normalizedRating.value)}`);

const quickFacts = computed<DetailRow[]>(() => {
  const rows: DetailRow[] = [];
  const format = props.book.format?.trim();
  const language = props.book.language?.trim();

  if (format) {
    rows.push({ label: t('bookDetail.fields.format'), value: format.toUpperCase() });
  }
  if (language) {
    rows.push({ label: t('bookDetail.fields.language'), value: formatLanguage(language) });
  }
  if (props.book.published_at) {
    rows.push({ label: t('bookDetail.fields.publishedAt'), value: formatDateLabel(props.book.published_at) });
  }

  return rows;
});

const publicationRows = computed<DetailRow[]>(() =>
  Object.entries(props.book.identifiers ?? {})
    .filter(([, value]) => value.trim().length > 0)
    .map(([key, value]) => ({
      label: key.toUpperCase(),
      value,
      className: 'identifier-value'
    }))
);

const contentRows = computed<DetailRow[]>(() => {
  const rows: DetailRow[] = [];
  if (typeof props.currentSource?.line_count === 'number') {
    rows.push({ label: t('bookDetail.fields.lines'), value: formatNumber(props.currentSource.line_count) });
  }
  if (typeof props.currentSource?.char_count === 'number') {
    rows.push({ label: t('bookDetail.fields.characters'), value: formatNumber(props.currentSource.char_count) });
  }
  return rows;
});

const noteRows = computed<DetailRow[]>(() => {
  const rows: DetailRow[] = [];
  const comment = props.book.comment?.trim();
  const importNotes = props.currentSource?.comment?.trim();

  if (comment) {
    rows.push({ label: t('bookDetail.fields.comment'), value: comment });
  }
  if (importNotes) {
    rows.push({ label: t('bookDetail.fields.importNotes'), value: importNotes });
  }
  return rows;
});

const hasDetailSections = computed(() =>
  chapters.value.length > 0 ||
  publicationRows.value.length > 0 ||
  contentRows.value.length > 0 ||
  noteRows.value.length > 0
);
</script>

<style scoped>
.detail-main {
  display: grid;
  gap: 22px;
  min-width: 0;
}

.detail-heading {
  display: grid;
  gap: 10px;
}

.detail-title {
  margin: 2px 0 0;
  max-width: 18ch;
  font-family: 'Noto Serif TC Variable', 'Noto Serif TC', Georgia, serif;
  font-size: clamp(32px, 4vw, 46px);
  font-weight: 720;
  letter-spacing: -0.035em;
  line-height: 1.12;
  text-wrap: balance;
}

.detail-authors {
  margin: 0;
  color: #556273;
  font-size: 17px;
  line-height: 1.5;
}

.summary-signals {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 10px 14px;
}

.rating-text {
  color: #d88a19;
  flex: none;
  font-size: 19px;
  letter-spacing: 0.08em;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.tag-pill {
  background: #f1efe9;
  border: 1px solid #e4e0d7;
  border-radius: 999px;
  color: #53606e;
  font-size: 12px;
  font-weight: 650;
  line-height: 1.4;
  padding: 4px 9px;
}

.quick-facts {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  margin: 4px 0 0;
}

.quick-fact {
  align-items: baseline;
  background: rgba(255, 255, 255, 0.62);
  border: 1px solid #e4e0d8;
  border-radius: 9px;
  display: inline-flex;
  gap: 6px;
  padding: 7px 9px;
}

.quick-fact dt,
.quick-fact dd {
  margin: 0;
}

.quick-fact dt {
  color: #788391;
  font-size: 11px;
  font-weight: 700;
}

.quick-fact dd {
  color: #334152;
  font-size: 13px;
  font-weight: 650;
}

.reading-slot:empty {
  display: none;
}

.detail-sections {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.detail-card {
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid #e5e1d9;
  border-radius: 14px;
  padding: 17px 18px;
}

.detail-card h3 {
  color: #283544;
  font-size: 14px;
  letter-spacing: 0.02em;
  margin: 0 0 12px;
}

.detail-card-notes,
.detail-card-chapters {
  grid-column: 1 / -1;
}

.chapter-list {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  list-style: none;
  margin: 0;
  padding: 0;
}

.chapter-item {
  align-items: baseline;
  background: none;
  border: 0;
  border-radius: 8px;
  color: #334152;
  cursor: pointer;
  display: flex;
  font: inherit;
  gap: 9px;
  padding: 7px 8px;
  text-align: left;
  width: 100%;
}

.chapter-item:hover,
.chapter-item:focus-visible {
  background: rgba(255, 255, 255, 0.9);
}

.chapter-item-index {
  color: #788391;
  flex: none;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  font-weight: 650;
  min-width: 2ch;
  text-align: right;
}

.chapter-item-title {
  font-size: 14px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.chapter-toggle {
  background: none;
  border: 0;
  color: #556273;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 650;
  justify-self: start;
  margin-top: 10px;
  padding: 6px 8px;
}

.chapter-toggle:hover,
.chapter-toggle:focus-visible {
  color: #283544;
}

.detail-definition-list {
  display: grid;
  gap: 10px;
  margin: 0;
}

.detail-definition-row {
  display: grid;
  gap: 4px;
  grid-template-columns: minmax(82px, 0.4fr) minmax(0, 1fr);
}

.detail-definition-row dt,
.detail-definition-row dd {
  margin: 0;
}

.detail-definition-row dt {
  color: #788391;
  font-size: 12px;
  font-weight: 650;
}

.detail-definition-row dd {
  color: #334152;
  font-size: 14px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.identifier-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  user-select: text;
}

.note-row {
  grid-template-columns: 1fr;
}

.note-row dd {
  white-space: pre-wrap;
}

.detail-empty {
  border-top: 1px solid #e5e1d9;
  color: #788391;
  grid-column: 1 / -1;
  margin: 0;
  padding-top: 16px;
  font-size: 13px;
}

@media (max-width: 768px) {
  .detail-main {
    display: contents;
  }

  .detail-heading {
    align-content: start;
    gap: 7px;
    grid-area: summary;
    min-width: 0;
  }

  .detail-title {
    font-size: clamp(24px, 7.2vw, 32px);
    line-height: 1.14;
    max-width: none;
  }

  .detail-authors {
    font-size: 14px;
  }

  .summary-signals {
    gap: 8px;
  }

  .rating-text {
    font-size: 15px;
  }

  .quick-facts {
    gap: 5px;
  }

  .quick-fact {
    padding: 5px 7px;
  }

  .quick-fact dt {
    display: none;
  }

  .quick-fact dd {
    font-size: 11px;
  }

  .reading-slot {
    grid-area: reading;
  }

  .detail-sections {
    grid-area: details;
    grid-template-columns: 1fr;
  }

  .detail-card-notes,
  .detail-card-chapters {
    grid-column: auto;
  }
}

@media (max-width: 360px) {
  .detail-heading {
    text-align: center;
  }

  .detail-heading :deep(.layer-breadcrumb),
  .summary-signals,
  .quick-facts {
    justify-content: center;
  }
}
</style>
