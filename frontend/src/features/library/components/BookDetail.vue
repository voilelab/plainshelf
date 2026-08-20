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
            <dd v-if="row.render === 'html'">
              <SafeHtml class="note-description" :html="row.html" profile="summary" />
            </dd>
            <dd v-else>{{ row.value }}</dd>
          </div>
        </dl>
      </section>

      <p v-if="!hasDetailSections" class="detail-empty">{{ t('bookDetail.emptyDetails') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import LayerBreadcrumb from './LayerBreadcrumb.vue';
import SafeHtml from '@/components/SafeHtml.vue';
import type { Book, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { formatLanguage } from '@/utils/language';
import { formatDateLabel } from '@/utils/date';
import { renderDescriptionHtml, toPlainSummary } from '@/utils/safeHtml';
import { useI18n } from '@/i18n';

const props = defineProps<{
  book: Book;
  progress?: ReadingProgress | null;
  currentSource?: SourceMeta | null;
}>();

const { t } = useI18n();

interface DetailRow {
  label: string;
  value: string;
  className?: string;
}

/**
 * The two notes rows differ in kind, not in label. A description is written by
 * a person in Markdown, or arrives from an EPUB import as the HTML the OPF
 * carried, and is rendered; an import note is prose this app generated to say
 * what the conversion could not keep, and stays the literal text it is.
 */
type NoteRow =
  | { label: string; render: 'text'; value: string }
  | { label: string; render: 'html'; html: string };

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

const noteRows = computed<NoteRow[]>(() => {
  const rows: NoteRow[] = [];
  const description = props.book.comment ?? '';
  const importNotes = props.currentSource?.comment?.trim();

  // A description earns a row when it amounts to words. Markup with no text in
  // it - `<br>`, an empty `<p>`, an image on its own - survives sanitizing as
  // an empty block, which is a labelled row with nothing under it.
  if (toPlainSummary(description)) {
    rows.push({
      label: t('bookDetail.fields.comment'),
      render: 'html',
      html: renderDescriptionHtml(description)
    });
  }
  if (importNotes) {
    rows.push({ label: t('bookDetail.fields.importNotes'), render: 'text', value: importNotes });
  }
  return rows;
});

const hasDetailSections = computed(() =>
  publicationRows.value.length > 0 || contentRows.value.length > 0 || noteRows.value.length > 0
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

.detail-card-notes {
  grid-column: 1 / -1;
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

/* The description is markup, so the newlines between its block tags belong to
   the renderer rather than to the text, and must not be preserved. */
.note-description {
  white-space: normal;
}

/* v-html descendants never receive the scoped-style attribute, so every
   selector crossing the safe HTML boundary is explicit, as in the reader. */
.note-description :deep(p),
.note-description :deep(ul),
.note-description :deep(ol),
.note-description :deep(dl),
.note-description :deep(pre),
.note-description :deep(blockquote) {
  margin: 0 0 8px;
}

.note-description :deep(h1),
.note-description :deep(h2),
.note-description :deep(h3),
.note-description :deep(h4),
.note-description :deep(h5),
.note-description :deep(h6) {
  color: #283544;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.4;
  margin: 12px 0 6px;
}

.note-description :deep(ul),
.note-description :deep(ol) {
  padding-left: 1.4em;
}

.note-description :deep(li) {
  margin: 0 0 2px;
}

.note-description :deep(blockquote) {
  border-left: 3px solid #ddd8cd;
  color: #556273;
  padding: 2px 0 2px 10px;
}

.note-description :deep(pre) {
  background: rgba(63, 53, 41, 0.06);
  border-radius: 8px;
  overflow-x: auto;
  padding: 9px 11px;
  white-space: pre;
}

.note-description :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.9em;
}

.note-description :deep(pre code) {
  font-size: inherit;
}

.note-description :deep(hr) {
  border: none;
  border-top: 1px solid #e5e1d9;
  margin: 10px 0;
}

/* dd opens no block-formatting context of its own here, so a margin on the
   first or last block escapes it and widens the row's own gap instead. */
.note-description :deep(> :first-child) {
  margin-top: 0;
}

.note-description :deep(> :last-child) {
  margin-bottom: 0;
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

  .detail-card-notes {
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
