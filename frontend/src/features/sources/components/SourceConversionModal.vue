<template>
  <ConfirmModal
    :open="open"
    :title="title"
    :confirm-text="t('sources.conversion.confirm')"
    :busy-text="t('sources.conversion.busy')"
    :busy="busy"
    :confirm-disabled="!preview.canSubmit"
    @cancel="emit('cancel')"
    @confirm="submit"
  >
    <form class="conversion-form" @submit.prevent="submit">
      <p>{{ description }}</p>

      <label v-if="kind === 'regex-md'" class="conversion-field">
        <span>{{ t('sources.conversion.patternLabel') }}</span>
        <input
          ref="primaryInput"
          v-model="pattern"
          class="input"
          type="text"
          :disabled="busy"
          autocomplete="off"
        >
        <small>{{ t('sources.conversion.patternHelp') }}</small>
      </label>

      <label v-else-if="kind === 'line-count-md'" class="conversion-field">
        <span>{{ t('sources.conversion.lineCountLabel') }}</span>
        <input
          ref="primaryInput"
          v-model="lineCount"
          class="input"
          type="number"
          min="1"
          step="1"
          :disabled="busy"
        >
      </label>

      <div class="conversion-preview" aria-live="polite">
        <strong>{{ t('sources.conversion.previewTitle') }}</strong>
        <p v-if="preview.error" class="conversion-error" role="alert">{{ preview.error }}</p>
        <template v-else>
          <p>{{ preview.summary }}</p>
          <pre>{{ preview.excerpt }}</pre>
        </template>
      </div>

      <label class="set-current-option">
        <input v-model="setCurrent" type="checkbox" :disabled="busy">
        {{ t('sources.conversion.setCurrent') }}
      </label>

      <p v-if="error" class="conversion-error" role="alert">{{ error }}</p>
    </form>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { scanMarkdownH2Headings } from '@/utils/markdownChapters';
import {
  markdownToPlainText,
  textToMarkdownByLineCount,
  textToMarkdownByRegex
} from '@/features/sources/utils/sourceConversions';
import { useI18n } from '@/i18n';

const { t } = useI18n();

export type SourceConversionKind =
  | 'manual-md'
  | 'regex-md'
  | 'line-count-md'
  | 'plain-text'
;

const props = withDefaults(defineProps<{
  open: boolean;
  kind: SourceConversionKind;
  sourceId: string;
  content: string;
  busy?: boolean;
  error?: string;
}>(), {
  busy: false,
  error: ''
});

const emit = defineEmits<{
  cancel: [];
  create: [payload: { content: string; format: 'txt' | 'md'; comment: string; setCurrent: boolean }];
}>();

const pattern = ref('^(Chapter\\s+.+)$');
const lineCount = ref('1000');
const setCurrent = ref(true);
const primaryInput = ref<HTMLInputElement | null>(null);

const title = computed(() => {
  switch (props.kind) {
    case 'manual-md': return t('sources.conversion.titles.manualMd');
    case 'regex-md': return t('sources.conversion.titles.regexMd');
    case 'line-count-md': return t('sources.conversion.titles.lineCountMd');
    case 'plain-text': return t('sources.conversion.titles.plainText');
  }
});

const description = computed(() => {
  switch (props.kind) {
    case 'manual-md': return t('sources.conversion.descriptions.manualMd');
    case 'regex-md': return t('sources.conversion.descriptions.regexMd');
    case 'line-count-md': return t('sources.conversion.descriptions.lineCountMd');
    case 'plain-text': return t('sources.conversion.descriptions.plainText');
  }
});

type ConversionPreview = {
  canSubmit: boolean;
  content: string;
  format: 'txt' | 'md';
  comment: string;
  summary: string;
  excerpt: string;
  error: string;
};

function excerpt(value: string): string {
  const normalized = value.slice(0, 800).trim();
  return value.length > 800 ? `${normalized}\n…` : normalized || t('sources.conversion.emptySource');
}

const preview = computed<ConversionPreview>(() => {
  try {
    let nextContent = props.content;
    let format: 'txt' | 'md' = 'md';
    let comment = '';
    let summary = '';

    switch (props.kind) {
      case 'manual-md': {
        comment = `Manual Markdown copy of ${props.sourceId}`;
        const chapters = scanMarkdownH2Headings(nextContent).length;
        summary = chapters === 1
          ? t('sources.conversion.summaries.manualMdOne')
          : t('sources.conversion.summaries.manualMdMany', { count: chapters });
        break;
      }
      case 'regex-md': {
        if (!pattern.value.trim()) throw new Error(t('sources.conversion.errors.emptyPattern'));
        const converted = textToMarkdownByRegex(props.content, pattern.value);
        if (converted.chapters === 0) {
          throw new Error(t('sources.conversion.errors.patternMatchedNothing'));
        }
        nextContent = converted.content;
        comment = `Regex chapter conversion of ${props.sourceId}: ${pattern.value}`;
        summary = t('sources.conversion.summaries.regexMd', { count: converted.chapters });
        break;
      }
      case 'line-count-md': {
        const size = Number(lineCount.value);
        if (!Number.isFinite(size) || size < 1) {
          throw new Error(t('sources.conversion.errors.invalidLineCount'));
        }
        const converted = textToMarkdownByLineCount(props.content, size);
        nextContent = converted.content;
        comment = `Fixed ${Math.trunc(size)} line conversion of ${props.sourceId}`;
        summary = t('sources.conversion.summaries.lineCountMd', { count: converted.chapters });
        break;
      }
      case 'plain-text': {
        nextContent = markdownToPlainText(props.content);
        format = 'txt';
        comment = `Plain-text conversion of ${props.sourceId}`;
        summary = t('sources.conversion.summaries.plainText');
        break;
      }
    }

    return {
      canSubmit: true,
      content: nextContent,
      format,
      comment,
      summary,
      excerpt: excerpt(nextContent),
      error: ''
    };
  } catch (err) {
    return {
      canSubmit: false,
      content: '',
      format: props.kind === 'plain-text' ? 'txt' : 'md',
      comment: '',
      summary: '',
      excerpt: '',
      error: err instanceof Error ? err.message : t('sources.conversion.errors.previewFailed')
    };
  }
});

function submit(): void {
  if (props.busy || !preview.value.canSubmit) return;
  emit('create', {
    content: preview.value.content,
    format: preview.value.format,
    comment: preview.value.comment,
    setCurrent: setCurrent.value
  });
}

watch(() => props.open, async (open) => {
  if (!open) return;
  pattern.value = '^(Chapter\\s+.+)$';
  lineCount.value = '1000';
  setCurrent.value = true;
  await nextTick();
  await nextTick();
  primaryInput.value?.focus();
  primaryInput.value?.select();
});
</script>

<style scoped>
.conversion-form,
.conversion-field,
.conversion-preview {
  display: grid;
  gap: 8px;
}

.conversion-form > p,
.conversion-preview p {
  margin: 0;
}

.conversion-field > span,
.conversion-preview > strong {
  color: var(--text);
  font-weight: 700;
}

.conversion-field small {
  color: var(--muted);
}

.conversion-preview {
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px;
}

.conversion-preview pre {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.45;
  margin: 0;
  max-height: 180px;
  overflow: auto;
  padding: 8px;
  white-space: pre-wrap;
  word-break: break-word;
}

.set-current-option {
  align-items: center;
  color: var(--text);
  display: flex;
  gap: 8px;
}

.conversion-error {
  color: #b91c1c;
  white-space: pre-line;
}
</style>
