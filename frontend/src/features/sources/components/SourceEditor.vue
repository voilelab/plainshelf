<template>
  <section class="editor-panel">
    <div class="source-editor-status" role="status">
      <p v-if="loading" class="meta">Loading source...</p>
      <p v-else-if="!sourceId" class="meta">Select a source to start editing.</p>
      <p v-else-if="dirty" class="meta dirty">Unsaved changes</p>
      <p v-else class="meta">No pending changes</p>
      <span v-if="sourceId && !loading" class="status-spacer"></span>
      <span v-if="sourceId && !loading && isCurrent" class="current-badge">Current</span>
      <button
        v-if="sourceId && !loading && !isCurrent"
        class="button set-current-btn"
        type="button"
        :disabled="settingCurrent"
        @click="$emit('setCurrent')"
      >
        {{ settingCurrent ? 'Setting...' : 'Set as current' }}
      </button>
    </div>

    <div class="editor-find-replace" role="group" aria-label="Find and replace">
      <label class="control-field">
        <span class="field-label">Find</span>
        <input
          v-model="findQuery"
          class="control-input"
          type="text"
          placeholder="Search text"
          :disabled="isEditorDisabled"
          @keydown.enter.prevent="findNext"
        />
      </label>

      <label class="control-field">
        <span class="field-label">Replace</span>
        <input
          v-model="replaceQuery"
          class="control-input"
          type="text"
          placeholder="Replace with"
          :disabled="isEditorDisabled"
          @keydown="onReplaceInputKeydown"
        />
      </label>

      <div class="find-actions">
        <button class="button" type="button" :disabled="disableFind" @click="findPrevious">Prev</button>
        <button class="button" type="button" :disabled="disableFind" @click="findNext">Next</button>
        <button class="button" type="button" :disabled="disableFind" @click="replaceNext">
          Replace
        </button>
        <button class="button" type="button" :disabled="disableFind" @click="replaceAll">
          Replace all
        </button>
      </div>
      <p class="find-status" role="status" aria-live="polite">{{ findStatus }}</p>
    </div>

    <div v-if="error" class="error editor-error" role="alert">{{ error }}</div>

    <textarea
      ref="textareaRef"
      class="source-content-textarea"
      :value="modelValue"
      :disabled="!sourceId || loading || saving"
      spellcheck="false"
      @input="onInput"
    ></textarea>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import type { SourceEditorAdapter } from '@/features/sources/types/editorAdapter';

const props = defineProps<{
  modelValue: string;
  sourceId: string;
  loading?: boolean;
  saving?: boolean;
  dirty?: boolean;
  error?: string;
  isCurrent?: boolean;
  settingCurrent?: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
  setCurrent: [];
}>();

const textareaRef = ref<HTMLTextAreaElement | null>(null);
const findQuery = ref('');
const replaceQuery = ref('');
const findStatus = ref('');

const isEditorDisabled = computed(() => !props.sourceId || props.loading || props.saving);
const disableFind = computed(() => isEditorDisabled.value || !findQuery.value);

watch(
  () => props.sourceId,
  () => {
    findQuery.value = '';
    replaceQuery.value = '';
    findStatus.value = '';
  }
);

watch(findQuery, () => {
  findStatus.value = '';
});

function onInput(event: Event): void {
  const target = event.target as HTMLTextAreaElement;
  emit('update:modelValue', target.value);
}

function findNext(): void {
  findMatch(false);
}

function findPrevious(): void {
  findMatch(true);
}

function matchOffsets(text: string, query: string): number[] {
  const offsets: number[] = [];
  let offset = 0;
  while (offset <= text.length - query.length) {
    const index = text.indexOf(query, offset);
    if (index === -1) break;
    offsets.push(index);
    offset = index + query.length;
  }
  return offsets;
}

function selectMatch(textarea: HTMLTextAreaElement, query: string, index: number): void {
  textarea.focus();
  textarea.setSelectionRange(index, index + query.length);
  scrollOffsetIntoView(textarea, index);
  const offsets = matchOffsets(textarea.value, query);
  const ordinal = offsets.indexOf(index) + 1;
  findStatus.value = ordinal > 0 ? `Match ${ordinal} of ${offsets.length}.` : `${offsets.length} matches.`;
}

function scrollOffsetIntoView(textarea: HTMLTextAreaElement, offset: number): void {
  const style = getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(style.lineHeight) || 27;
  const targetTop = measureTextareaOffsetTop(textarea, offset);
  const margin = Math.min(48, textarea.clientHeight / 4);
  const visibleTop = textarea.scrollTop + margin;
  const visibleBottom = textarea.scrollTop + textarea.clientHeight - lineHeight - margin;

  if (targetTop < visibleTop || targetTop > visibleBottom) {
    // WebKit/Wails does not consistently scroll a textarea when only its
    // programmatic selection changes. Assign after the current frame so this
    // wins over any focus/selection scroll restoration performed by WebKit.
    requestAnimationFrame(() => {
      textarea.scrollTop = Math.max(0, targetTop - textarea.clientHeight / 3);
    });
  }
}

function measureTextareaOffsetTop(textarea: HTMLTextAreaElement, offset: number): number {
  const style = getComputedStyle(textarea);
  const mirror = document.createElement('div');
  const marker = document.createElement('span');
  const copiedProperties = [
    'borderTopWidth',
    'borderRightWidth',
    'borderBottomWidth',
    'borderLeftWidth',
    'fontFamily',
    'fontSize',
    'fontStyle',
    'fontVariant',
    'fontWeight',
    'letterSpacing',
    'lineHeight',
    'paddingTop',
    'paddingRight',
    'paddingBottom',
    'paddingLeft',
    'tabSize',
    'textIndent',
    'textTransform',
    'wordBreak',
    'wordSpacing'
  ] as const;

  mirror.style.position = 'absolute';
  mirror.style.left = '-100000px';
  mirror.style.top = '0';
  mirror.style.visibility = 'hidden';
  mirror.style.pointerEvents = 'none';
  mirror.style.boxSizing = style.boxSizing;
  mirror.style.width = `${textarea.getBoundingClientRect().width}px`;
  mirror.style.height = 'auto';
  mirror.style.minHeight = '0';
  mirror.style.maxHeight = 'none';
  mirror.style.overflow = 'hidden';
  mirror.style.whiteSpace = style.whiteSpace;
  mirror.style.overflowWrap = style.overflowWrap;

  for (const property of copiedProperties) {
    mirror.style[property] = style[property];
  }

  const clampedOffset = Math.max(0, Math.min(textarea.value.length, offset));
  mirror.append(document.createTextNode(textarea.value.slice(0, clampedOffset)));
  marker.textContent = '\u200b';
  mirror.append(marker);
  document.body.append(mirror);
  const top = marker.offsetTop;
  mirror.remove();
  return top;
}

function findMatch(backward: boolean): number | null {
  const textarea = textareaRef.value;
  const query = findQuery.value;
  if (!textarea || !query || isEditorDisabled.value) {
    return null;
  }

  const text = textarea.value;
  let index = backward
    ? text.slice(0, textarea.selectionStart).lastIndexOf(query)
    : text.indexOf(query, textarea.selectionEnd);

  if (index === -1) {
    index = backward ? text.lastIndexOf(query) : text.indexOf(query);
  }

  if (index === -1) {
    findStatus.value = 'No matches.';
    return null;
  }

  selectMatch(textarea, query, index);
  return index;
}

function replaceNext(): void {
  const textarea = textareaRef.value;
  const query = findQuery.value;
  if (!textarea || !query || isEditorDisabled.value) {
    return;
  }

  let start = textarea.selectionStart;
  let end = textarea.selectionEnd;
  const selection = textarea.value.slice(start, end);
  if (selection !== query) {
    const match = findMatch(false);
    if (match === null) return;
    start = match;
    end = match + query.length;
  }

  const before = textarea.value.slice(0, start);
  const after = textarea.value.slice(end);
  const replaced = `${before}${replaceQuery.value}${after}`;
  const nextCursor = before.length + replaceQuery.value.length;
  emit('update:modelValue', replaced);
  void nextTick(() => {
    const current = textareaRef.value;
    if (!current) return;
    let nextMatch = current.value.indexOf(query, nextCursor);
    if (nextMatch === -1) nextMatch = current.value.indexOf(query);
    if (nextMatch === -1) {
      current.focus();
      current.setSelectionRange(nextCursor, nextCursor);
      scrollOffsetIntoView(current, nextCursor);
      findStatus.value = 'Replaced 1 occurrence. No matches remain.';
      return;
    }
    selectMatch(current, query, nextMatch);
    findStatus.value = `Replaced 1 occurrence. ${findStatus.value}`;
  });
}

function onReplaceInputKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.isComposing) {
    return;
  }

  event.preventDefault();
  replaceNext();
}

function replaceAll(): void {
  const query = findQuery.value;
  if (!query || isEditorDisabled.value) {
    return;
  }

  const source = textareaRef.value?.value ?? props.modelValue;
  const occurrences = matchOffsets(source, query).length;
  if (occurrences === 0) {
    findStatus.value = 'No matches.';
    return;
  }
  const replaced = source.split(query).join(replaceQuery.value);
  emit('update:modelValue', replaced);
  void nextTick(() => {
    const textarea = textareaRef.value;
    if (textarea) {
      const cursor = Math.min(textarea.selectionStart, textarea.value.length);
      textarea.focus();
      textarea.setSelectionRange(cursor, cursor);
    }
    findStatus.value = `Replaced ${occurrences} occurrence${occurrences === 1 ? '' : 's'}.`;
  });
}

function getCurrentParagraphStart(): number {
  const cursor = textareaRef.value?.selectionStart ?? 0;
  const before = props.modelValue.slice(0, cursor);
  const blankLine = /(?:^|\n)[ \t]*\r?\n/g;
  let start = 0;
  for (const match of before.matchAll(blankLine)) {
    start = (match.index ?? 0) + match[0].length;
  }
  return start;
}

function replaceRange(startOffset: number, endOffset: number, replacement: string): void {
  const start = Math.max(0, Math.min(props.modelValue.length, startOffset));
  const end = Math.max(start, Math.min(props.modelValue.length, endOffset));
  emit('update:modelValue', `${props.modelValue.slice(0, start)}${replacement}${props.modelValue.slice(end)}`);
  void nextTick(() => focusAndSelect(start, start + replacement.length));
}

function jumpToOffset(offset: number): void {
  focusAndSelect(offset, offset);
  const textarea = textareaRef.value;
  if (textarea) scrollOffsetIntoView(textarea, offset);
}

function focusAndSelect(startOffset: number, endOffset: number): void {
  const textarea = textareaRef.value;
  if (!textarea) return;
  const start = Math.max(0, Math.min(textarea.value.length, startOffset));
  const end = Math.max(start, Math.min(textarea.value.length, endOffset));
  textarea.focus();
  textarea.setSelectionRange(start, end);
}

defineExpose<SourceEditorAdapter>({
  getCurrentParagraphStart,
  replaceRange,
  jumpToOffset,
  focusAndSelect
});
</script>

<style scoped>
.editor-panel {
  flex: 1;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.source-editor-status {
  flex-shrink: 0;
  min-height: 38px;
  display: flex;
  align-items: center;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  background: #f8fafc;
}

.source-editor-status p {
  margin: 0;
}

.source-editor-status .dirty {
  color: #9a3412;
}

.status-spacer {
  flex: 1;
}

.current-badge {
  font-size: 12px;
  font-weight: 600;
  color: #166534;
  background: #dcfce7;
  border: 1px solid #bbf7d0;
  border-radius: 10px;
  padding: 2px 8px;
}

.set-current-btn {
  font-size: 12px;
  padding: 3px 10px;
}

.source-content-textarea {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  width: 100%;
  box-sizing: border-box;
  border: none;
  outline: none;
  background: #fff;
  color: var(--text);
  padding: 24px 32px;
  font-size: 16px;
  line-height: 1.7;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  resize: none;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.source-content-textarea:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 32%, transparent);
  outline-offset: -2px;
}

.editor-find-replace {
  flex-shrink: 0;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 8px 10px;
  align-items: end;
  background: #f8fafc;
}

.control-field {
  display: grid;
  gap: 4px;
}

.field-label {
  font-size: 12px;
  color: var(--muted);
}

.control-input {
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0 10px;
  font: inherit;
}

.find-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.find-status {
  color: var(--muted);
  font-size: 12px;
  grid-column: 1 / -1;
  margin: 0;
  min-height: 1.2em;
}

.editor-error {
  flex-shrink: 0;
  margin: 10px 12px;
}

@media (max-width: 900px) {
  .editor-find-replace {
    grid-template-columns: 1fr;
  }

  .find-actions {
    flex-wrap: wrap;
  }
}
</style>
