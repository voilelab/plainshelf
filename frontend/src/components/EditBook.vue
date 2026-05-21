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
          <span class="label">Tags (comma separated)</span>
          <input v-model="tagsInput" class="input" type="text" placeholder="fiction, webnovel" />
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
const tagsInput = ref('');
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
    tagsInput.value = listToCommaString(book.tags);
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
    tags: commaStringToList(tagsInput.value),
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
