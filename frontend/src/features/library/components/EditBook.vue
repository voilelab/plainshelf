<template>
  <article :class="['edit-panel', { panel: !embedded, 'edit-panel-embedded': embedded }]">
    <header v-if="!embedded" class="edit-header">
      <h2>{{ t('libraryForms.editBook.title') }}</h2>
      <p class="meta">{{ t('libraryForms.editBook.description') }}</p>
    </header>

    <form class="edit-form" :aria-busy="saving" @submit.prevent="onSubmit">
      <div class="edit-form-fields" :inert="saving ? true : undefined">
        <section class="section-block">
          <h3>{{ t('libraryForms.editBook.basicInfo') }}</h3>
          <label class="field">
            <span class="label">{{ t('libraryForms.editBook.titleLabel') }}</span>
            <input v-model="title" class="input" type="text" :placeholder="t('libraryForms.editBook.titlePlaceholder')" />
          </label>

          <label class="field">
            <span class="label">{{ t('libraryForms.editBook.authorsLabel') }}</span>
            <input v-model="authorsInput" class="input" type="text" :placeholder="t('libraryForms.editBook.authorsPlaceholder')" />
          </label>

        </section>

      <section class="section-block">
        <h3>{{ t('libraryForms.editBook.organization') }}</h3>
        <label class="field">
          <span class="label">{{ t('libraryForms.editBook.publishedAt') }}</span>
          <input v-model="publishedAtInput" class="input" type="date" />
        </label>

        <!-- Same row-as-label shape the settings panels use: the switch is a
             <button>, so `for` is what keeps the help text a click target. -->
        <div class="field">
          <label class="nsfw-row" :for="nsfwSwitchId">
            <div>
              <span :id="nsfwLabelId" class="label">{{ t('libraryForms.editBook.nsfw.label') }}</span>
              <p :id="nsfwHelpId" class="field-help">{{ nsfwHelpText }}</p>
            </div>
            <BaseSwitch
              :id="nsfwSwitchId"
              v-model="nsfwShown"
              :disabled="folderNsfwRule !== undefined"
              :aria-labelledby="nsfwLabelId"
              :aria-describedby="nsfwHelpId"
            />
          </label>
        </div>

        <fieldset class="field rating-field">
          <legend class="label">{{ t('libraryForms.editBook.starRating') }}</legend>
          <div class="star-rating">
            <RatingRoot v-model="star" as="div" class="star-rating-root" :length="5" clearable :aria-label="t('libraryForms.editBook.starRating')">
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
                  :aria-label="starLabel(value)"
                >
                  ★
                </RatingItemIndicator>
              </RatingItem>
            </RatingRoot>
            <button class="clear-rating" type="button" :disabled="star === 0" @click="star = 0">{{ t('libraryForms.editBook.clearRating') }}</button>
          </div>
        </fieldset>

        <label class="field">
          <span class="label">{{ t('libraryForms.editBook.languageLabel') }}</span>
          <SelectRoot :model-value="languageSelectValue" @update:model-value="onLanguageSelect">
            <SelectTrigger class="input select select-trigger">
              <SelectValue />
            </SelectTrigger>
            <SelectPortal>
              <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                <SelectViewport>
                  <SelectItem
                    v-for="option in languageSelectItems"
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
            :placeholder="t('language.book.customPlaceholder')"
          />
          <p class="field-help">{{ t('language.book.help') }}</p>
          <p v-if="languageError" class="error field-error">{{ languageError }}</p>
        </label>

        <label class="field">
          <span class="label">{{ t('libraryForms.editBook.tags') }}</span>
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
              <TagsInputItemDelete class="tag-remove" :aria-label="t('libraryForms.editBook.removeTag', { tag })">×</TagsInputItemDelete>
            </TagsInputItem>
            <TagsInputInput ref="tagsInputRef" class="tag-input" :placeholder="t('libraryForms.editBook.tagsPlaceholder')" />
          </TagsInputRoot>
          <p class="field-help">{{ t('libraryForms.editBook.tagsHelp') }}</p>
        </label>

        <div class="field">
          <label class="label" :for="commentFieldId">{{ t('libraryForms.editBook.comment') }}</label>
          <textarea
            :id="commentFieldId"
            v-model="comment"
            class="input textarea"
            rows="5"
            :placeholder="t('libraryForms.editBook.commentPlaceholder')"
          ></textarea>
          <p class="field-help">{{ t('libraryForms.editBook.commentHelp') }}</p>
          <CollapsibleRoot v-model:open="showCommentPreview" class="comment-preview-collapsible">
            <CollapsibleTrigger class="comment-preview-toggle">
              {{
                showCommentPreview
                  ? t('libraryForms.editBook.commentPreviewHide')
                  : t('libraryForms.editBook.commentPreviewShow')
              }}
            </CollapsibleTrigger>
            <CollapsibleContent
              class="comment-preview"
              role="region"
              :aria-label="t('libraryForms.editBook.commentPreviewLabel')"
            >
              <SafeHtml
                v-if="commentPreviewHtml"
                class="description-body"
                :html="commentPreviewHtml"
                profile="summary"
              />
              <p v-else class="comment-preview-empty">
                {{ t('libraryForms.editBook.commentPreviewEmpty') }}
              </p>
            </CollapsibleContent>
          </CollapsibleRoot>
        </div>

        <div class="field">
          <span class="label">{{ t('libraryForms.editBook.identifiers') }}</span>
          <div class="identifier-rows">
            <div v-for="(row, index) in identifierRows" :key="index" class="identifier-row">
              <input
                v-model="row.key"
                class="input identifier-key"
                type="text"
                :placeholder="t('libraryForms.editBook.identifierKeyPlaceholder')"
                :aria-label="t('libraryForms.editBook.identifierKeyLabel', { index: index + 1 })"
              />
              <input
                v-model="row.value"
                class="input identifier-value"
                type="text"
                :placeholder="t('libraryForms.editBook.identifierValuePlaceholder')"
                :aria-label="t('libraryForms.editBook.identifierValueLabel', { index: index + 1 })"
              />
              <button
                class="identifier-remove"
                type="button"
                :aria-label="t('libraryForms.editBook.removeIdentifier', { name: row.key || index + 1 })"
                @click="removeIdentifierRow(index)"
              >
                ×
              </button>
            </div>
          </div>
          <button class="button" type="button" @click="addIdentifierRow">{{ t('libraryForms.editBook.addIdentifier') }}</button>
        </div>
        </section>
      </div>

      <p v-if="error" class="error submit-error">{{ error }}</p>

      <div class="form-actions">
        <button class="button primary" type="submit" :disabled="saving">
          {{ saving ? t('libraryForms.editBook.saving') : t('libraryForms.editBook.save') }}
        </button>
        <button class="button" type="button" :disabled="saving" @click="emit('cancel')">{{ t('common.cancel') }}</button>
      </div>
    </form>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue';
import {
  CollapsibleContent,
  CollapsibleRoot,
  CollapsibleTrigger,
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
import BaseSwitch from '@/components/BaseSwitch.vue';
import SafeHtml from '@/components/SafeHtml.vue';
import type { Book, BookUpdateRequest } from '@/types/book';
import {
  CUSTOM_LANGUAGE_VALUE,
  LANGUAGE_VALUES,
  isValidLanguageTag,
  languageSelectOptions,
  normalizeLanguage
} from '@/utils/language';
import { commaStringToList, listToCommaString } from '@/utils/metadata';
import { renderDescriptionHtml } from '@/utils/safeHtml';
import { useI18n } from '@/i18n';

const { t } = useI18n();

// English says "1 star" but "2 stars"; the catalog has no plural rules, so the
// two forms are separate keys.
function starLabel(value: number): string {
  return value === 1
    ? t('libraryForms.editBook.starValueOne')
    : t('libraryForms.editBook.starValueMany', { count: value });
}

// The custom sentinel is not one of these — it only ever exists as a Select
// choice — so the preset list needs no guard against it beyond dropping the
// empty "unspecified" entry.
const COMMON_LANGUAGE_VALUES: Set<string> = new Set(
  LANGUAGE_VALUES.filter((value) => value)
);
const STAR_VALUES = [1, 2, 3, 4, 5] as const;
// reka-ui SelectItem forbids an empty-string value (it's reserved to mean
// "clear selection / show placeholder"), but languageSelectOptions() uses ''
// for "unspecified". Map it to this sentinel for the Select only; the
// underlying languagePreset ref keeps using '' so the custom-language v-if
// and watchers below are untouched.
const EMPTY_LANGUAGE_SELECT_VALUE = '__unspecified__';

const props = defineProps<{
  book: Book;
  saving: boolean;
  error?: string;
  embedded?: boolean;
}>();

const emit = defineEmits<{
  (event: 'submit', payload: BookUpdateRequest): void;
  (event: 'cancel'): void;
  (event: 'dirty-change', dirty: boolean): void;
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
// A flag, not the message. Holding translated text here would leave a shown
// error stranded in the locale it was produced in while the placeholder, help
// text and options around it follow a switch.
const languageTagInvalid = ref(false);
const languageError = computed(() => (languageTagInvalid.value ? t('language.book.invalidTag') : ''));
const comment = ref('');
const commentFieldId = useId();
const showCommentPreview = ref(false);
// The same render the detail page runs, so what is previewed here and what is
// shown there are one output of one function; `SafeHtml` sanitizes it under the
// same `summary` profile. The textarea keeps the source text either way - a
// preview never changes what is submitted.
const commentPreviewHtml = computed(() => renderDescriptionHtml(comment.value));
const publishedAtInput = ref('');
const star = ref(0);
// The book's own half of the adult-content mark. The folder rule is the other
// half and is not editable here, so a folder-marked book shows the switch on
// and disabled — a control that could be turned off without the book becoming
// visible would be a lie about what the shelf does.
const nsfw = ref(false);
const nsfwSwitchId = useId();
const nsfwLabelId = useId();
const nsfwHelpId = useId();
const folderNsfwRule = computed(() => props.book.nsfw_folder);
// What the switch renders is the whole mark; what the payload carries is only
// the book's own half. They differ for a folder-marked book, and keeping them
// one value would either show it as unmarked or write the folder's mark into
// its book.json, where clearing the folder rule would then leave it behind.
const nsfwShown = computed<boolean>({
  get: () => folderNsfwRule.value !== undefined || nsfw.value,
  set: (value) => {
    nsfw.value = value;
  }
});
const nsfwHelpText = computed(() => {
  const rule = folderNsfwRule.value;
  if (!rule) {
    return t('libraryForms.editBook.nsfw.help');
  }
  return rule.reason
    ? t('libraryForms.editBook.nsfw.fromFolderReason', { path: rule.path, reason: rule.reason })
    : t('libraryForms.editBook.nsfw.fromFolder', { path: rule.path });
});
const identifierRows = ref<{ key: string; value: string }[]>([]);
const initialDraft = ref('');
// languageSelectOptions() resolves its labels through t(), so reading it inside
// a computed is what keeps them following a locale change.
const languageSelectItems = computed(() =>
  languageSelectOptions().map((option) => ({
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
    languageTagInvalid.value = false;
    comment.value = book.comment ?? '';
    publishedAtInput.value = toFormDateValue(book.published_at);
    star.value = normalizeStar(book.star);
    // A folder rule already marks the book, so the switch reads on whatever
    // book.json says; the payload below still sends the book's own value, so
    // saving cannot silently write the folder's mark into the book.
    nsfw.value = book.nsfw === true;
    identifierRows.value = Object.entries(book.identifiers ?? {}).map(([key, value]) => ({ key, value }));
    initialDraft.value = serializeDraft();
  },
  { immediate: true }
);

const isDirty = computed(() => serializeDraft() !== initialDraft.value);

watch(isDirty, (dirty) => emit('dirty-change', dirty), { immediate: true });

watch(languagePreset, (nextPreset) => {
  if (nextPreset !== CUSTOM_LANGUAGE_VALUE) {
    languageTagInvalid.value = false;
  }
});

watch(customLanguage, () => {
  if (languageTagInvalid.value) {
    languageTagInvalid.value = false;
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

function serializeDraft(): string {
  return JSON.stringify({
    title: title.value,
    authorsInput: authorsInput.value,
    tags: tagsSource.value,
    languagePreset: languagePreset.value,
    customLanguage: customLanguage.value,
    comment: comment.value,
    publishedAtInput: publishedAtInput.value,
    star: star.value,
    nsfw: nsfw.value,
    identifierRows: identifierRows.value
  });
}

function onSubmit(): void {
  const rawLanguage = languagePreset.value === CUSTOM_LANGUAGE_VALUE ? customLanguage.value : languagePreset.value;
  if (languagePreset.value === CUSTOM_LANGUAGE_VALUE && !isValidLanguageTag(rawLanguage)) {
    languageTagInvalid.value = true;
    return;
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
    nsfw: nsfw.value,
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

.edit-form-fields {
  display: grid;
  gap: 14px;
}

.edit-form-fields[inert] {
  opacity: 0.72;
}

.edit-panel-embedded {
  max-width: none;
  margin: 0;
  padding: 0;
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

.nsfw-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.nsfw-row .field-help {
  margin-top: 4px;
}

.field-error {
  margin: 0;
}

.textarea {
  resize: vertical;
  min-height: 120px;
}

/* The collapsible only groups the trigger and its panel for reka; it must not
   become a grid item of its own, or the toggle and preview would share one
   cell instead of stacking as the surrounding fields do. */
.comment-preview-collapsible {
  display: contents;
}

.comment-preview-toggle {
  justify-self: start;
  padding: 0;
  border: 0;
  background: none;
  color: var(--accent);
  cursor: pointer;
  font-size: 12px;
}

.comment-preview-toggle:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

/* A window of its own, not a box the content sizes. The height is fixed rather
   than bounded: any range between a min and a max is still a box that grows a
   line at a time, and everything below it - the identifiers, the save button -
   moves down as the description is typed. An empty preview showing empty space
   is the price of the field under it staying where it was. */
.comment-preview {
  height: 180px;
  overflow-y: auto;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: #fff;
  color: var(--text);
  font-size: 14px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.comment-preview-empty {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.submit-error {
  margin: 0;
}

.form-actions {
  display: flex;
  gap: 8px;
}

.edit-panel-embedded .form-actions {
  position: sticky;
  bottom: 0;
  z-index: 1;
  padding: 12px 0 2px;
  background: var(--surface);
  box-shadow: 0 -10px 16px -16px rgba(15, 23, 42, 0.45);
}

@media (max-width: 720px) {
  .edit-panel {
    padding: 14px;
  }

  .form-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 520px) {
  .identifier-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .identifier-key,
  .identifier-value {
    grid-column: 1;
  }

  .identifier-remove {
    grid-column: 2;
    grid-row: 1 / 3;
  }

  .edit-panel-embedded .form-actions .button {
    flex: 1 1 140px;
  }
}
</style>

<style scoped src="@/styles/description.css"></style>
