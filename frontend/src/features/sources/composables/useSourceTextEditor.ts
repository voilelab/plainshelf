import { computed, nextTick, ref, watch } from 'vue';
import {
  clampTextOffset,
  findMatchOffset,
  matchOffsets,
  paragraphStartOffset,
  replaceAllText,
  replaceTextRange
} from '@/features/sources/utils/textEditing';

interface SourceTextEditorOptions {
  content: () => string;
  sourceId: () => string;
  disabled: () => boolean;
  updateContent: (value: string) => void;
}

export function useSourceTextEditor(options: SourceTextEditorOptions) {
  const textareaRef = ref<HTMLTextAreaElement | null>(null);
  const findQuery = ref('');
  const replaceQuery = ref('');
  const findStatus = ref('');
  const disableFind = computed(() => options.disabled() || !findQuery.value);

  watch(options.sourceId, () => {
    findQuery.value = '';
    replaceQuery.value = '';
    findStatus.value = '';
  });

  watch(findQuery, () => {
    findStatus.value = '';
  });

  function onInput(event: Event): void {
    options.updateContent((event.target as HTMLTextAreaElement).value);
  }

  function selectMatch(textarea: HTMLTextAreaElement, query: string, index: number): void {
    textarea.focus();
    textarea.setSelectionRange(index, index + query.length);
    scrollOffsetIntoView(textarea, index);
    const offsets = matchOffsets(textarea.value, query);
    const ordinal = offsets.indexOf(index) + 1;
    findStatus.value = ordinal > 0
      ? `Match ${ordinal} of ${offsets.length}.`
      : `${offsets.length} matches.`;
  }

  function findMatch(backward: boolean): number | null {
    const textarea = textareaRef.value;
    const query = findQuery.value;
    if (!textarea || !query || options.disabled()) return null;

    const index = findMatchOffset(
      textarea.value,
      query,
      textarea.selectionStart,
      textarea.selectionEnd,
      backward
    );
    if (index === null) {
      findStatus.value = 'No matches.';
      return null;
    }

    selectMatch(textarea, query, index);
    return index;
  }

  function findNext(): void {
    findMatch(false);
  }

  function findPrevious(): void {
    findMatch(true);
  }

  function replaceNext(): void {
    const textarea = textareaRef.value;
    const query = findQuery.value;
    if (!textarea || !query || options.disabled()) return;

    let start = textarea.selectionStart;
    let end = textarea.selectionEnd;
    if (textarea.value.slice(start, end) !== query) {
      const match = findMatch(false);
      if (match === null) return;
      start = match;
      end = match + query.length;
    }

    const edit = replaceTextRange(textarea.value, start, end, replaceQuery.value);
    const nextCursor = edit.selectionEnd;
    options.updateContent(edit.value);
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
    if (event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    replaceNext();
  }

  function replaceAll(): void {
    const query = findQuery.value;
    if (!query || options.disabled()) return;

    const textarea = textareaRef.value;
    const result = replaceAllText(textarea?.value ?? options.content(), query, replaceQuery.value);
    if (result.occurrences === 0) {
      findStatus.value = 'No matches.';
      return;
    }

    const previousCursor = textarea?.selectionStart ?? 0;
    options.updateContent(result.value);
    void nextTick(() => {
      const current = textareaRef.value;
      if (current) {
        const cursor = clampTextOffset(current.value, previousCursor);
        current.focus();
        current.setSelectionRange(cursor, cursor);
      }
      findStatus.value = `Replaced ${result.occurrences} occurrence${result.occurrences === 1 ? '' : 's'}.`;
    });
  }

  function getCurrentParagraphStart(): number {
    return paragraphStartOffset(options.content(), textareaRef.value?.selectionStart ?? 0);
  }

  function replaceRange(startOffset: number, endOffset: number, replacement: string): void {
    const edit = replaceTextRange(options.content(), startOffset, endOffset, replacement);
    options.updateContent(edit.value);
    void nextTick(() => focusAndSelect(edit.selectionStart, edit.selectionEnd));
  }

  function jumpToOffset(offset: number): void {
    focusAndSelect(offset, offset);
    const textarea = textareaRef.value;
    if (textarea) scrollOffsetIntoView(textarea, offset);
  }

  function focusAndSelect(startOffset: number, endOffset: number): void {
    const textarea = textareaRef.value;
    if (!textarea) return;
    const start = clampTextOffset(textarea.value, startOffset);
    const end = Math.max(start, clampTextOffset(textarea.value, endOffset));
    textarea.focus();
    textarea.setSelectionRange(start, end);
  }

  return {
    textareaRef,
    findQuery,
    replaceQuery,
    findStatus,
    disableFind,
    onInput,
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
