<template>
  <article class="panel edit-panel">
    <header class="edit-header">
      <h2>Edit metadata</h2>
      <p class="meta">Update fields supported by the current API.</p>
    </header>

    <form class="edit-form" @submit.prevent="onSubmit">
      <section class="section-block">
        <h3>Basic info</h3>
        <label class="field">
          <span class="label">Title</span>
          <input v-model="title" class="input" type="text" placeholder="Book title" />
        </label>

        <label class="field">
          <span class="label">Authors (comma separated)</span>
          <input v-model="authorsInput" class="input" type="text" placeholder="Author A, Author B" />
        </label>
      </section>

      <section class="section-block">
        <h3>Organization</h3>
        <label class="field">
          <span class="label">Published At</span>
          <input v-model="publishedAtInput" class="input" type="date" />
        </label>

        <fieldset class="field rating-field">
          <legend class="label">Star rating</legend>
          <div class="star-rating">
            <RatingRoot v-model="star" as="div" class="star-rating-root" :length="5" clearable aria-label="Star rating">
              <RatingItem
                v-for="value in STAR_VALUES"
                :key="value"
                :item="value"
                as="span"
                class="star-item"
                v-slot="{ steps }"
              >
                <RatingItemIndicator
                  v-for="step in steps"
                  :key="step"
                  :step="step"
                  class="star-indicator"
                  :aria-label="`${value} star${value === 1 ? '' : 's'}`"
                >
                  ★
                </RatingItemIndicator>
              </RatingItem>
            </RatingRoot>
            <button class="clear-rating" type="button" :disabled="star === 0" @click="star = 0">Clear</button>
          </div>
        </fieldset>

        <label class="field">
          <span class="label">Language</span>
          <SelectRoot :model-value="languageSelectValue" @update:model-value="onLanguageSelect">
            <SelectTrigger class="input select select-trigger">
              <SelectValue />
            </SelectTrigger>
            <SelectPortal>
              <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                <SelectViewport>
                  <SelectItem
                    v-for="option in languageSelectOptions"
                    :key="option.value"
                    class="reka-menu-item"
                    :value="option.value"
                  >
                    <SelectItemText>{{ option.label }}</SelectItemText>
                  </SelectItem>
                </SelectViewport>
              </SelectContent>
            </SelectPortal>
          </SelectRoot>
          <input
            v-if="languagePreset === CUSTOM_LANGUAGE_VALUE"
            v-model="customLanguage"
            class="input"
            type="text"
            placeholder="例如 zh-TW, zh-HK, fr, de"
          />
          <p class="field-help">建議使用 en、ja、ko、zh-Hant、zh-Hans；也可填 zh-TW 這類 BCP 47 language tag。</p>
          <p v-if="languageError" class="error field-error">{{ languageError }}</p>
        </label>

        <label class="field">
          <span class="label">Tags</span>
          <TagsInputRoot
            v-model="tags"
            class="tag-input-shell"
            add-on-blur
            add-on-paste
            :convert-value="normalizeTag"
            @click="focusTagInput"
          >
            <TagsInputItem v-for="tag in tags" :key="tag" :value="tag" class="tag-chip">
              <TagsInputItemText />
              <TagsInputItemDelete class="tag-remove" :aria-label="`Remove tag ${tag}`">×</TagsInputItemDelete>
            </TagsInputItem>
            <TagsInputInput ref="tagsInputRef" class="tag-input" placeholder="Type a tag and press Enter" />
          </TagsInputRoot>
          <p class="field-help">Press Enter or comma to add tags. Click × to remove.</p>
        </label>

        <label class="field">
          <span class="label">Comment</span>
          <textarea
            v-model="comment"
            class="input textarea"
            rows="5"
            placeholder="Notes about this book"
          ></textarea>
        </label>

        <div class="field">
          <span class="label">Identifiers</span>
          <div class="identifier-rows">
            <div v-for="(row, index) in identifierRows" :key="index" class="identifier-row">
              <input
                v-model="row.key"
                class="input identifier-key"
                type="text"
                placeholder="isbn"
                :aria-label="`Identifier key ${index + 1}`"
              />
              <input
                v-model="row.value"
                class="input identifier-value"
                type="text"
                placeholder="9787020002207"
                :aria-label="`Identifier value ${index + 1}`"
              />
              <button
                class="identifier-remove"
                type="button"
                :aria-label="`Remove identifier ${row.key || index + 1}`"
                @click="removeIdentifierRow(index)"
              >
                ×
              </button>
            </div>
          </div>
          <button class="button" type="button" @click="addIdentifierRow">Add identifier</button>
        </div>
      </section>

      <p v-if="error" class="error submit-error">{{ error }}</p>

      <div class="form-actions">
        <button class="button primary" type="submit" :disabled="saving">
          {{ saving ? 'Saving...' : 'Save metadata' }}
        </button>
        <button class="button" type="button" :disabled="saving" @click="emit('cancel')">Cancel</button>
      </div>
    </form>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
  RatingItem,
  RatingItemIndicator,
  RatingRoot,
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  TagsInputInput,
  TagsInputItem,
  TagsInputItemDelete,
  TagsInputItemText,
  TagsInputRoot,
  type AcceptableValue
} from 'reka-ui';
import type { Book, BookUpdateRequest } from '../types/book';
import {
  CUSTOM_LANGUAGE_VALUE,
  LANGUAGE_OPTIONS,
  LANGUAGE_SELECT_OPTIONS,
  normalizeLanguage,
  validateLanguageTag
} from '../utils/language';
import { commaStringToList, listToCommaString } from '../utils/metadata';

const COMMON_LANGUAGE_VALUES: Set<string> = new Set(
  LANGUAGE_OPTIONS.map((option) => option.value).filter((value) => value && value !== CUSTOM_LANGUAGE_VALUE)
);
const STAR_VALUES = [1, 2, 3, 4, 5] as const;
// reka-ui SelectItem forbids an empty-string value (it's reserved to mean
// "clear selection / show placeholder"), but LANGUAGE_SELECT_OPTIONS uses ''
// for "unspecified". Map it to this sentinel for the Select only; the
// underlying languagePreset ref keeps using '' so the custom-language v-if
// and watchers below are untouched.
const EMPTY_LANGUAGE_SELECT_VALUE = '__unspecified__';

const props = defineProps<{
  book: Book;
  saving: boolean;
  error?: string;
}>();

const emit = defineEmits<{
  (event: 'submit', payload: BookUpdateRequest): void;
  (event: 'cancel'): void;
}>();

const title = ref('');
const authorsInput = ref('');
const tagsSource = ref<string[]>([]);
const tags = computed<string[]>({
  get: () => tagsSource.value,
  set: (next) => {
    tagsSource.value = next.filter((tag) => tag.length > 0);
  }
});
const tagsInputRef = ref<InstanceType<typeof TagsInputInput> | null>(null);
const languagePreset = ref('');
const customLanguage = ref('');
const languageError = ref('');
const comment = ref('');
const publishedAtInput = ref('');
const star = ref(0);
const identifierRows = ref<{ key: string; value: string }[]>([]);
const languageSelectOptions = computed(() =>
  LANGUAGE_SELECT_OPTIONS.map((option) => ({
    value: option.value === '' ? EMPTY_LANGUAGE_SELECT_VALUE : option.value,
    label: option.label
  }))
);
const languageSelectValue = computed<string>({
  get: () => (languagePreset.value === '' ? EMPTY_LANGUAGE_SELECT_VALUE : languagePreset.value),
  set: (value) => {
    languagePreset.value = value === EMPTY_LANGUAGE_SELECT_VALUE ? '' : value;
  }
});

watch(
  () => props.book,
  (book) => {
    title.value = book.title;
    authorsInput.value = listToCommaString(book.authors);
    tags.value = commaStringToList(listToCommaString(book.tags));
    const initialLanguage = (book.language ?? '').trim();
    if (initialLanguage === '') {
      languagePreset.value = '';
      customLanguage.value = '';
    } else if (COMMON_LANGUAGE_VALUES.has(initialLanguage)) {
      languagePreset.value = initialLanguage;
      customLanguage.value = '';
    } else {
      languagePreset.value = CUSTOM_LANGUAGE_VALUE;
      customLanguage.value = initialLanguage;
    }
    languageError.value = '';
    comment.value = book.comment ?? '';
    publishedAtInput.value = toFormDateValue(book.published_at);
    star.value = normalizeStar(book.star);
    identifierRows.value = Object.entries(book.identifiers ?? {}).map(([key, value]) => ({ key, value }));
  },
  { immediate: true }
);

watch(languagePreset, (nextPreset) => {
  if (nextPreset !== CUSTOM_LANGUAGE_VALUE) {
    languageError.value = '';
  }
});

watch(customLanguage, () => {
  if (languageError.value) {
    languageError.value = '';
  }
});

function onLanguageSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    languageSelectValue.value = value;
  }
}

function normalizeTag(rawValue: string): string {
  return rawValue.trim().replace(/\s+/g, ' ');
}

function focusTagInput(event: MouseEvent): void {
  if (event.target !== event.currentTarget) {
    return;
  }
  (tagsInputRef.value as unknown as { $el?: HTMLInputElement } | null)?.$el?.focus();
}

function addIdentifierRow(): void {
  identifierRows.value.push({ key: '', value: '' });
}

function removeIdentifierRow(index: number): void {
  identifierRows.value.splice(index, 1);
}

function buildIdentifiersPayload(): Record<string, string> {
  const entries = identifierRows.value
    .map((row) => [row.key.trim(), row.value] as const)
    .filter(([key]) => key.length > 0);
  return Object.fromEntries(entries);
}

function onSubmit(): void {
  const rawLanguage = languagePreset.value === CUSTOM_LANGUAGE_VALUE ? customLanguage.value : languagePreset.value;
  if (languagePreset.value === CUSTOM_LANGUAGE_VALUE) {
    const errorMessage = validateLanguageTag(rawLanguage);
    if (errorMessage) {
      languageError.value = errorMessage;
      return;
    }
  }

  const normalizedLanguage = normalizeLanguage(rawLanguage);

  emit('submit', {
    title: title.value.trim(),
    authors: commaStringToList(authorsInput.value),
    tags: tags.value,
    language: normalizedLanguage || '',
    comment: comment.value.trim(),
    published_at: publishedAtInput.value || undefined,
    star: star.value,
    identifiers: buildIdentifiersPayload()
  });
}

function normalizeStar(rawValue: unknown): number {
  if (typeof rawValue !== 'number' || !Number.isFinite(rawValue)) {
    return 0;
  }
  return Math.min(5, Math.max(0, Math.trunc(rawValue)));
}

function toFormDateValue(rawValue?: string): string {
  if (!rawValue) {
    return '';
  }
  // The HTML date input wants exactly "YYYY-MM-DD". The API already returns
  // date-only values, so this slice is a no-op for them and a safety net if a
  // full timestamp ever slips through.
  return rawValue.slice(0, 10);
}
</script>

<style scoped>
.edit-panel {
  max-width: 760px;
  margin: 0 auto;
  padding: 16px;
}

.edit-header {
  margin-bottom: 12px;
}

.edit-header h2 {
  margin: 0;
}

.edit-form {
  display: grid;
  gap: 14px;
}

.section-block {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #fcfdff;
}

.section-block h3 {
  margin: 0;
  font-size: 16px;
}

.field {
  display: grid;
  gap: 6px;
}

.label {
  color: var(--muted);
  font-size: 13px;
}

.select {
  background-color: #fff;
}

.select-trigger {
  cursor: pointer;
  text-align: left;
}

.rating-field {
  margin: 0;
  padding: 0;
  border: 0;
}

.star-rating {
  display: flex;
  align-items: center;
  gap: 4px;
}

.star-rating-root {
  display: flex;
  align-items: center;
  gap: 4px;
}

.star-item {
  display: inline-flex;
}

/* :deep() required: reka-ui's Radio renders a fragment, which breaks scoped
   scope-id inheritance, so the actual star <button> never gets our data-v attr. */
.star-rating :deep(.star-indicator) {
  padding: 0 2px;
  border: 0;
  background: transparent;
  color: #c4cad4;
  cursor: pointer;
  font-size: 28px;
  line-height: 1;
}

.star-rating :deep(.star-indicator[data-state='active']) {
  color: #f5a623;
}

.star-rating :deep(.star-indicator:focus-visible),
.clear-rating:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.clear-rating {
  margin-left: 8px;
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.clear-rating:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.tag-input-shell {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #fff;
}

.tag-input-shell:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(82, 102, 255, 0.12);
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 999px;
  background: #eef2ff;
  color: #2b3a9a;
  font-size: 13px;
}

.tag-chip[data-state='active'] {
  background: #dbeafe;
  outline: 1px solid #93c5fd;
}

.tag-remove {
  border: none;
  background: transparent;
  color: inherit;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  padding: 0;
}

.tag-input {
  flex: 1 1 180px;
  min-width: 140px;
  border: none;
  outline: none;
  background: transparent;
  font: inherit;
  color: inherit;
  padding: 4px 0;
}

.identifier-rows {
  display: grid;
  gap: 8px;
}

.identifier-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.identifier-key {
  flex: 1 1 160px;
  min-width: 0;
}

.identifier-value {
  flex: 2 1 240px;
  min-width: 0;
}

.identifier-remove {
  flex: 0 0 auto;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 4px;
}

.identifier-remove:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.field-help {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.field-error {
  margin: 0;
}

.textarea {
  resize: vertical;
  min-height: 120px;
}

.submit-error {
  margin: 0;
}

.form-actions {
  display: flex;
  gap: 8px;
}

@media (max-width: 720px) {
  .edit-panel {
    padding: 14px;
  }

  .form-actions {
    flex-wrap: wrap;
  }
}
</style>
