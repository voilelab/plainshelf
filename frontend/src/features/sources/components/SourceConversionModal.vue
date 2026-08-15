<template>
  <ConfirmModal
    :open="open"
    :title="title"
    confirm-text="Create source"
    cancel-text="Cancel"
    busy-text="Creating..."
    :busy="busy"
    :confirm-disabled="!preview.canSubmit"
    @cancel="emit('cancel')"
    @confirm="submit"
  >
    <form class="conversion-form" @submit.prevent="submit">
      <p>{{ description }}</p>

      <label v-if="kind === 'regex-md'" class="conversion-field">
        <span>Chapter title regular expression</span>
        <input
          ref="primaryInput"
          v-model="pattern"
          class="input"
          type="text"
          :disabled="busy"
          autocomplete="off"
        >
        <small>Capture group 1 becomes the H2 title. Without a capture group, the full match is used.</small>
      </label>

      <label v-else-if="kind === 'line-count-md'" class="conversion-field">
        <span>Lines per chapter</span>
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
        <strong>Preview</strong>
        <p v-if="preview.error" class="conversion-error" role="alert">{{ preview.error }}</p>
        <template v-else>
          <p>{{ preview.summary }}</p>
          <pre>{{ preview.excerpt }}</pre>
        </template>
      </div>

      <label class="set-current-option">
        <input v-model="setCurrent" type="checkbox" :disabled="busy">
        Set the new source as current
      </label>

      <p v-if="error" class="conversion-error" role="alert">{{ error }}</p>
    </form>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { scanMarkdownH2Headings } from '@/features/reader/utils/markdownChapters';
import type { SplitConfig } from '@/types/book';
import {
  markdownToPlainText,
  textToMarkdownByLineCount,
  textToMarkdownByRegex,
  upgradeLegacyToMarkdown
} from '@/features/sources/utils/sourceConversions';

export type SourceConversionKind =
  | 'manual-md'
  | 'regex-md'
  | 'line-count-md'
  | 'plain-text'
  | 'legacy-upgrade';

const props = withDefaults(defineProps<{
  open: boolean;
  kind: SourceConversionKind;
  sourceId: string;
  content: string;
  legacyConfig?: SplitConfig;
  busy?: boolean;
  error?: string;
}>(), {
  legacyConfig: () => ({ type: 'none' }),
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
    case 'manual-md': return 'Create chapterized Markdown source';
    case 'regex-md': return 'Convert title lines to chapters';
    case 'line-count-md': return 'Split into fixed-size chapters';
    case 'plain-text': return 'Create plain-text source';
    case 'legacy-upgrade': return 'Upgrade chapter format';
  }
});

const description = computed(() => {
  switch (props.kind) {
    case 'manual-md': return 'Copy the current TXT source into a Markdown draft. You can add H2 chapters in the source editor after creation.';
    case 'regex-md': return 'Matching title lines will be rewritten as Markdown H2 headings.';
    case 'line-count-md': return 'An H2 “Part N” heading will be inserted at every previewed boundary.';
    case 'plain-text': return 'Markdown markers will be removed. Heading hierarchy and chapter navigation will be lost.';
    case 'legacy-upgrade': return 'The current legacy split result will become H2 chapters. The original source remains unchanged.';
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
  return value.length > 800 ? `${normalized}\n…` : normalized || '(empty source)';
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
        summary = `The Markdown draft contains ${chapters} H2 chapter${chapters === 1 ? '' : 's'} initially.`;
        break;
      }
      case 'regex-md': {
        if (!pattern.value.trim()) throw new Error('Enter a regular expression.');
        const converted = textToMarkdownByRegex(props.content, pattern.value);
        if (converted.chapters === 0) throw new Error('The regular expression matched no chapter title lines.');
        nextContent = converted.content;
        comment = `Regex chapter conversion of ${props.sourceId}: ${pattern.value}`;
        summary = `${converted.chapters} H2 chapter headings will be created.`;
        break;
      }
      case 'line-count-md': {
        const size = Number(lineCount.value);
        if (!Number.isFinite(size) || size < 1) throw new Error('Lines per chapter must be a positive number.');
        const converted = textToMarkdownByLineCount(props.content, size);
        nextContent = converted.content;
        comment = `Fixed ${Math.trunc(size)} line conversion of ${props.sourceId}`;
        summary = `${converted.chapters} H2 chapter headings will be inserted.`;
        break;
      }
      case 'plain-text': {
        nextContent = markdownToPlainText(props.content);
        format = 'txt';
        comment = `Plain-text conversion of ${props.sourceId}`;
        summary = 'A single unstructured TXT section will be created.';
        break;
      }
      case 'legacy-upgrade': {
        nextContent = upgradeLegacyToMarkdown(props.content, props.legacyConfig);
        const chapters = scanMarkdownH2Headings(nextContent).length;
        comment = `Legacy chapter upgrade of ${props.sourceId}`;
        summary = `${chapters} H2 chapter${chapters === 1 ? '' : 's'} will be created from the current legacy split result.`;
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
      error: err instanceof Error ? err.message : 'Unable to preview this conversion.'
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
