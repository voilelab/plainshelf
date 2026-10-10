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
          <StarRatingInput v-model="star" />
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
          <TagInput v-model="tags" />
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
          <IdentifierRows v-model="identifierRows" />
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
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
import BaseSwitch from '@/components/BaseSwitch.vue';
import SafeHtml from '@/components/SafeHtml.vue';
import IdentifierRows, { type IdentifierRow } from '@/features/library/components/IdentifierRows.vue';
import StarRatingInput from '@/features/library/components/StarRatingInput.vue';
import TagInput from '@/features/library/components/TagInput.vue';
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

// The custom sentinel is not one of these — it only ever exists as a Select
// choice — so the preset list needs no guard against it beyond dropping the
// empty "unspecified" entry.
const COMMON_LANGUAGE_VALUES: Set<string> = new Set(
  LANGUAGE_VALUES.filter((value) => value)
);
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
const identifierRows = ref<IdentifierRow[]>([]);
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
  .edit-panel-embedded .form-actions .button {
    flex: 1 1 140px;
  }
}
</style>

<style scoped src="@/styles/description.css"></style>
