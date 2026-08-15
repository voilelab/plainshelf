import { computed, nextTick, ref, watch } from 'vue';
import {
  clampTextOffset,
  findMatchOffset,
  mapOffsetThroughReplaceAll,
  matchOffsets,
  paragraphStartOffset,
  replaceAllText,
  replaceTextRange
} from '@/features/sources/utils/textEditing';
import type {
  SourceDocumentEdit,
  SourceEditorViewRange,
  SourceFindScope
} from '@/features/sources/types/editorAdapter';

interface SourceTextEditorOptions {
  content: () => string;
  sourceId: () => string;
  disabled: () => boolean;
  viewRange: () => SourceEditorViewRange | null;
  findScope: () => SourceFindScope;
  updateDocument: (edit: SourceDocumentEdit) => void;
  requestViewOffset: (offset: number, affinity: 'forward' | 'backward') => void;
}

interface NormalizedRange {
  startOffset: number;
  endOffset: number;
}

export function useSourceTextEditor(options: SourceTextEditorOptions) {
  const textareaRef = ref<HTMLTextAreaElement | null>(null);
  const findQuery = ref('');
  const replaceQuery = ref('');
  const findStatus = ref('');
  const disableFind = computed(() => options.disabled() || !findQuery.value);
  const visibleContent = computed(() => {
    const range = currentViewRange();
    return options.content().slice(range.startOffset, range.endOffset);
  });
  let composing = false;

  watch(options.sourceId, () => {
    findQuery.value = '';
    replaceQuery.value = '';
    findStatus.value = '';
  });

  watch([findQuery, options.findScope], () => {
    findStatus.value = '';
  });

  function currentViewRange(): NormalizedRange {
    const content = options.content();
    const requested = options.viewRange();
    if (!requested) return { startOffset: 0, endOffset: content.length };
    const startOffset = clampTextOffset(content, requested.startOffset);
    return {
      startOffset,
      endOffset: Math.max(startOffset, clampTextOffset(content, requested.endOffset))
    };
  }

  function currentScopeRange(): NormalizedRange {
    if (options.findScope() === 'section' && options.viewRange()) return currentViewRange();
    return { startOffset: 0, endOffset: options.content().length };
  }

  function documentSelection(): { start: number; end: number } {
    const view = currentViewRange();
    const textarea = textareaRef.value;
    return {
      start: view.startOffset + (textarea?.selectionStart ?? 0),
      end: view.startOffset + (textarea?.selectionEnd ?? 0)
    };
  }

  function syncTextarea(textarea: HTMLTextAreaElement, isComposing: boolean): void {
    const range = currentViewRange();
    const replacement = textarea.value;
    const edit = replaceTextRange(
      options.content(),
      range.startOffset,
      range.endOffset,
      replacement
    );
    const atVisibleEnd = textarea.selectionEnd === replacement.length;
    const selectionStart = range.startOffset + textarea.selectionStart;
    const selectionEnd = range.startOffset + textarea.selectionEnd;
    options.updateDocument({
      value: edit.value,
      selectionStart,
      selectionEnd,
      affinity: atVisibleEnd ? 'backward' : 'forward',
      deferView: true,
      composing: isComposing
    });
    if (!isComposing) {
      void nextTick(() => focusAndSelect(selectionStart, selectionEnd, false));
    }
  }

  function onInput(event: Event): void {
    const inputEvent = event as InputEvent;
    syncTextarea(
      inputEvent.target as HTMLTextAreaElement,
      composing || inputEvent.isComposing
    );
  }

  function onCompositionStart(): void {
    composing = true;
  }

  function onCompositionEnd(event: CompositionEvent): void {
    composing = false;
    // WebKit normally follows compositionend with a final non-composing input,
    // but synchronizing here also covers engines that do not. Replacing the
    // current projection with the same textarea value is idempotent.
    syncTextarea(event.target as HTMLTextAreaElement, false);
  }

  function findMatch(backward: boolean): number | null {
    const query = findQuery.value;
    if (!query || options.disabled()) return null;

    const content = options.content();
    const scope = currentScopeRange();
    const selection = documentSelection();
    const index = findMatchOffset(
      content.slice(scope.startOffset, scope.endOffset),
      query,
      selection.start - scope.startOffset,
      selection.end - scope.startOffset,
      backward
    );
    if (index === null) {
      findStatus.value = 'No matches.';
      return null;
    }
    return scope.startOffset + index;
  }

  async function selectMatch(index: number): Promise<void> {
    const query = findQuery.value;
    if (!query) return;
    options.requestViewOffset(index, 'forward');
    await nextTick();
    await nextTick();
    focusAndSelect(index, index + query.length, false);

    const scope = currentScopeRange();
    const offsets = matchOffsets(
      options.content().slice(scope.startOffset, scope.endOffset),
      query
    );
    const ordinal = offsets.indexOf(index - scope.startOffset) + 1;
    findStatus.value = ordinal > 0
      ? `Match ${ordinal} of ${offsets.length}.`
      : `${offsets.length} matches.`;
  }

  function findNext(): void {
    const index = findMatch(false);
    if (index !== null) void selectMatch(index);
  }

  function findPrevious(): void {
    const index = findMatch(true);
    if (index !== null) void selectMatch(index);
  }

  async function replaceNext(): Promise<void> {
    const query = findQuery.value;
    if (!query || options.disabled()) return;

    const selection = documentSelection();
    let start = selection.start;
    let end = selection.end;
    if (options.content().slice(start, end) !== query) {
      const match = findMatch(false);
      if (match === null) return;
      start = match;
      end = match + query.length;
    }

    const edit = replaceTextRange(options.content(), start, end, replaceQuery.value);
    options.updateDocument({
      value: edit.value,
      selectionStart: edit.selectionEnd,
      selectionEnd: edit.selectionEnd,
      affinity: 'forward'
    });
    await nextTick();

    const scope = currentScopeRange();
    const nextIndex = findMatchOffset(
      options.content().slice(scope.startOffset, scope.endOffset),
      query,
      edit.selectionEnd - scope.startOffset,
      edit.selectionEnd - scope.startOffset,
      false
    );
    if (nextIndex === null) {
      focusAndSelect(edit.selectionEnd, edit.selectionEnd);
      findStatus.value = 'Replaced 1 occurrence. No matches remain.';
      return;
    }
    await selectMatch(scope.startOffset + nextIndex);
    findStatus.value = `Replaced 1 occurrence. ${findStatus.value}`;
  }

  function onReplaceInputKeydown(event: KeyboardEvent): void {
    if (event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    void replaceNext();
  }

  function replaceAll(): void {
    const query = findQuery.value;
    if (!query || options.disabled()) return;

    const content = options.content();
    const scope = currentScopeRange();
    const scopedContent = content.slice(scope.startOffset, scope.endOffset);
    const result = replaceAllText(scopedContent, query, replaceQuery.value);
    if (result.occurrences === 0) {
      findStatus.value = 'No matches.';
      return;
    }

    const selection = documentSelection();
    const relativeCursor = clampTextOffset(scopedContent, selection.start - scope.startOffset);
    const mappedCursor = scope.startOffset + mapOffsetThroughReplaceAll(
      scopedContent,
      query,
      replaceQuery.value,
      relativeCursor
    );
    const nextValue = `${content.slice(0, scope.startOffset)}${result.value}${content.slice(scope.endOffset)}`;
    options.updateDocument({
      value: nextValue,
      selectionStart: mappedCursor,
      selectionEnd: mappedCursor,
      affinity: 'forward'
    });
    void nextTick(() => {
      focusAndSelect(mappedCursor, mappedCursor);
      findStatus.value = `Replaced ${result.occurrences} occurrence${result.occurrences === 1 ? '' : 's'}.`;
    });
  }

  function getCurrentParagraphStart(): number {
    return paragraphStartOffset(options.content(), documentSelection().start);
  }

  function replaceRange(startOffset: number, endOffset: number, replacement: string): void {
    const edit = replaceTextRange(options.content(), startOffset, endOffset, replacement);
    options.updateDocument({
      value: edit.value,
      selectionStart: edit.selectionStart,
      selectionEnd: edit.selectionEnd,
      affinity: 'forward'
    });
    void nextTick(() => focusAndSelect(edit.selectionStart, edit.selectionEnd));
  }

  function jumpToOffset(offset: number): void {
    focusAndSelect(offset, offset);
  }

  function focusAndSelect(
    startOffset: number,
    endOffset: number,
    requestView = true
  ): void {
    if (requestView) options.requestViewOffset(startOffset, 'forward');
    const textarea = textareaRef.value;
    if (!textarea) return;
    const view = currentViewRange();
    const start = clampTextOffset(textarea.value, startOffset - view.startOffset);
    const end = Math.max(start, clampTextOffset(textarea.value, endOffset - view.startOffset));
    textarea.focus();
    textarea.setSelectionRange(start, end);
    scrollOffsetIntoView(textarea, start);
  }

  return {
    textareaRef,
    visibleContent,
    findQuery,
    replaceQuery,
    findStatus,
    disableFind,
    onInput,
    onCompositionStart,
    onCompositionEnd,
    findNext,
    findPrevious,
    replaceNext,
    onReplaceInputKeydown,
    replaceAll,
    getCurrentParagraphStart,
    replaceRange,
    jumpToOffset,
    focusAndSelect
  };
}

function scrollOffsetIntoView(textarea: HTMLTextAreaElement, offset: number): void {
  const style = getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(style.lineHeight) || 27;
  const targetTop = measureTextareaOffsetTop(textarea, offset);
  const margin = Math.min(48, textarea.clientHeight / 4);
  const visibleTop = textarea.scrollTop + margin;
  const visibleBottom = textarea.scrollTop + textarea.clientHeight - lineHeight - margin;

  if (targetTop < visibleTop || targetTop > visibleBottom) {
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

  for (const property of copiedProperties) mirror.style[property] = style[property];

  const clampedOffset = clampTextOffset(textarea.value, offset);
  mirror.append(document.createTextNode(textarea.value.slice(0, clampedOffset)));
  marker.textContent = '\u200b';
  mirror.append(marker);
  document.body.append(mirror);
  const top = marker.offsetTop;
  mirror.remove();
  return top;
}
