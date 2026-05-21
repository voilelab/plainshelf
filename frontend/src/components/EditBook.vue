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
          <input v-model="publishedAtInput" class="input" type="datetime-local" />
        </label>

        <label class="field">
          <span class="label">Language</span>
          <select v-model="languagePreset" class="input select">
            <option v-for="option in LANGUAGE_SELECT_OPTIONS" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
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
          <div class="tag-input-shell" @click="focusTagInput">
            <ul v-if="tags.length" class="tag-list" aria-label="Current tags">
              <li v-for="tag in tags" :key="tag" class="tag-chip">
                <span>{{ tag }}</span>
                <button
                  class="tag-remove"
                  type="button"
                  :aria-label="`Remove tag ${tag}`"
                  @click.stop="removeTag(tag)"
                >
                  ×
                </button>
              </li>
            </ul>
            <input
              ref="tagsInputRef"
              v-model="tagDraft"
              class="tag-input"
              type="text"
              placeholder="Type a tag and press Enter"
              @keydown="onTagKeyDown"
              @blur="commitTagDraft"
            />
          </div>
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
import { ref, watch } from 'vue';
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
const tags = ref<string[]>([]);
const tagDraft = ref('');
const tagsInputRef = ref<HTMLInputElement | null>(null);
const languagePreset = ref('');
const customLanguage = ref('');
const languageError = ref('');
const comment = ref('');
const publishedAtInput = ref('');

watch(
  () => props.book,
  (book) => {
    title.value = book.title;
    authorsInput.value = listToCommaString(book.authors);
    tags.value = commaStringToList(listToCommaString(book.tags));
    tagDraft.value = '';
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
    publishedAtInput.value = toDatetimeLocalValue(book.published_at);
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

function normalizeTag(rawValue: string): string {
  return rawValue.trim().replace(/\s+/g, ' ');
}

function addTag(rawValue: string): void {
  const normalized = normalizeTag(rawValue);
  if (!normalized || tags.value.includes(normalized)) {
    return;
  }

  tags.value = [...tags.value, normalized];
}

function commitTagDraft(): void {
  const rawDraft = tagDraft.value;
  if (!rawDraft.trim()) {
    tagDraft.value = '';
    return;
  }

  const parts = rawDraft.split(',');
  parts.forEach((part) => addTag(part));
  tagDraft.value = '';
}

function removeTag(tagToRemove: string): void {
  tags.value = tags.value.filter((tag) => tag !== tagToRemove);
}

function focusTagInput(): void {
  tagsInputRef.value?.focus();
}

function onTagKeyDown(event: KeyboardEvent): void {
  if (event.isComposing || event.key === 'Process') {
    return;
  }

  if (event.key === 'Enter' || event.key === ',') {
    event.preventDefault();
    commitTagDraft();
    return;
  }

  if (event.key === 'Backspace' && !tagDraft.value && tags.value.length) {
    tags.value = tags.value.slice(0, -1);
  }
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
  commitTagDraft();

  emit('submit', {
    title: title.value.trim(),
    authors: commaStringToList(authorsInput.value),
    tags: tags.value,
    language: normalizedLanguage || '',
    comment: comment.value.trim(),
    published_at: fromDatetimeLocalValue(publishedAtInput.value)
  });
}

function toDatetimeLocalValue(rawValue?: string): string {
  if (!rawValue) {
    return '';
  }
  const date = new Date(rawValue);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  const localTime = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return localTime.toISOString().slice(0, 16);
}

function fromDatetimeLocalValue(rawValue: string): string | undefined {
  if (!rawValue) {
    return undefined;
  }
  const date = new Date(rawValue);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return `${date.toISOString().slice(0, 19)}Z`;
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

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
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
