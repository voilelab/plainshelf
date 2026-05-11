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
          <span class="label">Language</span>
          <select v-model="languagePreset" class="input select">
            <option v-for="option in LANGUAGE_OPTIONS" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
          <input
            v-if="languagePreset === 'custom'"
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
import { commaStringToList, listToCommaString } from '../utils/metadata';

const LANGUAGE_OPTIONS = [
  { value: '', label: '未指定' },
  { value: 'zh-Hant', label: '中文（繁體）' },
  { value: 'zh-Hans', label: '中文（簡體）' },
  { value: 'ja', label: '日文' },
  { value: 'ko', label: '韓文' },
  { value: 'en', label: '英文' },
  { value: 'custom', label: '自訂...' }
] as const;

const COMMON_LANGUAGE_VALUES: Set<string> = new Set(
  LANGUAGE_OPTIONS.map((option) => option.value).filter((value) => value && value !== 'custom')
);

const languageTagRE = /^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$/;

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
      languagePreset.value = 'custom';
      customLanguage.value = initialLanguage;
    }
    languageError.value = '';
    comment.value = book.comment ?? '';
  },
  { immediate: true }
);

watch(languagePreset, (nextPreset) => {
  if (nextPreset !== 'custom') {
    languageError.value = '';
  }
});

watch(customLanguage, () => {
  if (languageError.value) {
    languageError.value = '';
  }
});

function validateLanguageTag(input: string): string | null {
  const value = input.trim();

  if (value === '') return null;

  if (!languageTagRE.test(value)) {
    return '語言格式不正確，請使用 en、ja、zh-Hant、zh-TW 這類格式。';
  }

  return null;
}

function normalizeLanguage(input: string): string {
  const value = input.trim();

  const map: Record<string, string> = {
    'zh-tw': 'zh-Hant',
    'zh-hk': 'zh-Hant',
    'zh-mo': 'zh-Hant',
    'zh-hant': 'zh-Hant',
    'zh-cn': 'zh-Hans',
    'zh-sg': 'zh-Hans',
    'zh-hans': 'zh-Hans'
  };

  return map[value.toLowerCase()] ?? value;
}

function onSubmit(): void {
  const rawLanguage = languagePreset.value === 'custom' ? customLanguage.value : languagePreset.value;
  if (languagePreset.value === 'custom') {
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
    comment: comment.value.trim()
  });
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