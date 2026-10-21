import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue';
import {
  Compartment,
  EditorSelection,
  EditorState,
  StateEffect,
  StateField,
  Text,
  type Extension
} from '@codemirror/state';
import {
  Decoration,
  EditorView,
  ViewPlugin,
  keymap,
  type DecorationSet,
  type ViewUpdate
} from '@codemirror/view';
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { markdown, markdownLanguage } from '@codemirror/lang-markdown';
import { tags } from '@lezer/highlight';
import {
  SearchQuery,
  findNext as cmFindNext,
  findPrevious as cmFindPrevious,
  replaceAll as cmReplaceAll,
  replaceNext as cmReplaceNext,
  search,
  setSearchQuery
} from '@codemirror/search';
import { t } from '@/i18n';
import { paragraphStartOffset } from '@/features/sources/utils/textEditing';
import type {
  SourceDocumentEdit,
  SourceEditorViewRange,
  SourceFindScope
} from '@/features/sources/types/editorAdapter';

const DOCUMENT_SYNC_INTERVAL_MS = 150;

interface SourceCodeMirrorOptions {
  content: () => string;
  sourceId: () => string;
  disabled: () => boolean;
  viewRange: () => SourceEditorViewRange | null;
  findScope: () => SourceFindScope;
  format: () => 'txt' | 'md';
  contentLabel: () => string;
  updateDocument: (edit: SourceDocumentEdit) => void;
}

type FindState =
  | { kind: 'noMatches' }
  | { kind: 'invalidRegexp' }
  | { kind: 'match'; ordinal: number; total: number }
  | { kind: 'count'; total: number }
  | { kind: 'replaced'; count: number }
  | { kind: 'replacedOneNoneRemain' }
  | { kind: 'replacedOneThenMatch'; ordinal: number; total: number }
  | { kind: 'replacedOneThenCount'; total: number };

function renderFindState(state: FindState | null): string {
  if (!state) {
    return '';
  }

  switch (state.kind) {
    case 'noMatches':
      return t('sources.editor.find.noMatches');
    case 'invalidRegexp':
      return t('sources.editor.find.invalidRegexp');
    case 'match':
      return t('sources.editor.find.matchOrdinal', { ordinal: state.ordinal, total: state.total });
    case 'count':
      return t('sources.editor.find.matchCount', { total: state.total });
    case 'replaced':
      return state.count === 1
        ? t('sources.editor.find.replacedOne')
        : t('sources.editor.find.replacedMany', { count: state.count });
    case 'replacedOneNoneRemain':
      return t('sources.editor.find.replacedOneNoneRemain');
    case 'replacedOneThenMatch':
      return t('sources.editor.find.replacedOneThenMatch', {
        ordinal: state.ordinal,
        total: state.total
      });
    case 'replacedOneThenCount':
      return t('sources.editor.find.replacedOneThenCount', { total: state.total });
  }
}

// CodeMirror state 6.7.6 positions count line breaks as one unit; lineSeparator controls serialization, so CRLF source offsets differ from document positions.
function lineSeparatorOf(text: string): string {
  if (!text.includes('\r')) return '\n';
  const mixed = /(?<!\r)\n/.test(text) || /\r(?!\n)/.test(text);
  return mixed ? '\n' : '\r\n';
}

function splitLines(text: string, separator: string): string[] {
  return separator === '\r\n' ? text.split('\r\n') : text.split(/\r\n?|\n/);
}

const setSectionRange = StateEffect.define<SourceEditorViewRange | null>();

const sectionRangeField = StateField.define<SourceEditorViewRange | null>({
  create: () => null,
  update(value, tr) {
    let next = value;
    for (const effect of tr.effects) {
      if (effect.is(setSectionRange)) next = effect.value;
    }
    if (!next) return null;
    if (!tr.docChanged) return next;
    const startOffset = tr.changes.mapPos(next.startOffset, -1);
    const endOffset = tr.changes.mapPos(next.endOffset, 1);
    return {
      startOffset: Math.min(startOffset, endOffset),
      endOffset: Math.max(startOffset, endOffset)
    };
  }
});

const dimmedMark = Decoration.mark({ class: 'cm-source-dimmed' });

const sectionDecorations = EditorView.decorations.compute([sectionRangeField], (state) => {
  const range = state.field(sectionRangeField);
  if (!range) return Decoration.none;
  const start = Math.max(0, Math.min(state.doc.length, range.startOffset));
  const end = Math.max(start, Math.min(state.doc.length, range.endOffset));
  const ranges = [];
  if (start > 0) ranges.push(dimmedMark.range(0, start));
  if (end < state.doc.length) ranges.push(dimmedMark.range(end, state.doc.length));
  return Decoration.set(ranges);
});

// CodeMirror search 6.7.2 only highlights search matches while its search panel is present (searchPanelOpen in dist/index.js).
const setActiveQuery = StateEffect.define<SearchQuery | null>();

const activeQueryField = StateField.define<SearchQuery | null>({
  create: () => null,
  update(value, tr) {
    let next = value;
    for (const effect of tr.effects) {
      if (effect.is(setActiveQuery)) next = effect.value;
    }
    return next;
  }
});

function queryOf(state: EditorState): SearchQuery | null {
  return state.field(activeQueryField);
}

const HIGHLIGHT_MARGIN = 250;

const matchMark = Decoration.mark({ class: 'cm-searchMatch' });
const selectedMatchMark = Decoration.mark({ class: 'cm-searchMatch cm-searchMatch-selected' });

function buildMatchDecorations(view: EditorView): DecorationSet {
  const query = queryOf(view.state);
  if (!query?.valid) return Decoration.none;

  const selection = view.state.selection.main;
  const ranges: ReturnType<typeof matchMark.range>[] = [];
  let lastTo = -1;
  for (const visible of view.visibleRanges) {
    const from = Math.max(0, visible.from - HIGHLIGHT_MARGIN);
    const to = Math.min(view.state.doc.length, visible.to + HIGHLIGHT_MARGIN);
    const cursor = query.getCursor(view.state, from, to);
    for (let step = cursor.next(); !step.done; step = cursor.next()) {
      const match = step.value;
      if (match.from === match.to || match.from < lastTo) continue;
      lastTo = match.to;
      const selected = match.from === selection.from && match.to === selection.to;
      ranges.push((selected ? selectedMatchMark : matchMark).range(match.from, match.to));
    }
  }
  return Decoration.set(ranges, true);
}

const matchHighlighter = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;

    constructor(view: EditorView) {
      this.decorations = buildMatchDecorations(view);
    }

    update(update: ViewUpdate) {
      const queryChanged = update.startState.field(activeQueryField) !== update.state.field(activeQueryField);
      const sectionChanged =
        update.startState.field(sectionRangeField) !== update.state.field(sectionRangeField);
      if (
        queryChanged ||
        sectionChanged ||
        update.docChanged ||
        update.selectionSet ||
        update.viewportChanged
      ) {
        this.decorations = buildMatchDecorations(update.view);
      }
    }
  },
  { decorations: (plugin) => plugin.decorations }
);

const markdownHighlight = HighlightStyle.define([
  { tag: tags.heading, fontWeight: '700' },
  { tag: tags.strong, fontWeight: '700' },
  { tag: tags.emphasis, fontStyle: 'italic' },
  { tag: tags.strikethrough, textDecoration: 'line-through' },
  { tag: tags.quote, color: '#4b5c72', fontStyle: 'italic' },
  { tag: [tags.link, tags.url], color: '#1f6feb' },
  { tag: tags.monospace, color: '#9d174d' },
  { tag: [tags.processingInstruction, tags.contentSeparator, tags.escape, tags.character],
    color: '#94a3b8' },
  { tag: [tags.tagName, tags.angleBracket], color: '#0f766e' },
  { tag: tags.attributeName, color: '#a16207' },
  { tag: tags.attributeValue, color: '#b45309' },
  { tag: tags.comment, color: '#94a3b8', fontStyle: 'italic' }
]);

const markdownExtension: Extension = [
  markdown({ base: markdownLanguage }),
  syntaxHighlighting(markdownHighlight)
];

const editorTheme = EditorView.theme({
  '&': {
    height: '100%',
    backgroundColor: '#fff',
    color: 'var(--text)',
    fontSize: '16px'
  },
  '&.cm-focused': {
    outline: '2px solid color-mix(in srgb, var(--accent) 32%, transparent)',
    outlineOffset: '-2px'
  },
  '.cm-scroller': {
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace",
    lineHeight: '1.7'
  },
  '.cm-content': {
    padding: '24px 32px',
    caretColor: 'var(--text)'
  },
  // CSS Color opacity applies to the entire subtree; child syntax-span colors cannot override ancestor opacity.
  '.cm-source-dimmed': {
    opacity: '0.38'
  },
  '.cm-searchMatch': {
    backgroundColor: '#fde68a'
  },
  '.cm-searchMatch-selected': {
    backgroundColor: '#fb923c'
  }
});

const scrollMargins = EditorView.scrollMargins.of(() => ({ top: 24, bottom: 24 }));

function searchPhrases(): Extension {
  return EditorState.phrases.of({
    'current match': t('sources.editor.find.announce.currentMatch'),
    'on line': t('sources.editor.find.announce.onLine'),
    'replaced match on line $': t('sources.editor.find.announce.replacedOnLine'),
    'replaced $ matches': t('sources.editor.find.announce.replacedMatches')
  });
}

export function useSourceCodeMirror(options: SourceCodeMirrorOptions) {
  const hostRef = ref<HTMLElement | null>(null);
  // Vue 3.5.43 deep refs proxy objects; CodeMirror view 6.43.13 requires transaction.startState identity to match the current state.
  const view = shallowRef<EditorView | null>(null);

  const findQuery = ref('');
  const replaceQuery = ref('');
  const caseSensitive = ref(false);
  const useRegexp = ref(false);
  const wholeWord = ref(false);
  const findState = ref<FindState | null>(null);
  const findStatus = computed(() => renderFindState(findState.value));
  const disableFind = computed(() => options.disabled() || !findQuery.value);

  const languageCompartment = new Compartment();
  const editableCompartment = new Compartment();
  const labelCompartment = new Compartment();
  const phraseCompartment = new Compartment();

  let lineSeparator = '\n';
  let syncedDocument = '';
  let documentStale = false;
  let syncTimer: ReturnType<typeof setTimeout> | null = null;
  let syncPending = false;

  function editableExtension(): Extension {
    const disabled = options.disabled();
    return [EditorState.readOnly.of(disabled), EditorView.editable.of(!disabled)];
  }

  function languageExtension(): Extension {
    return options.format() === 'md' ? markdownExtension : [];
  }

  function labelExtension(): Extension {
    return EditorView.contentAttributes.of({ 'aria-label': options.contentLabel() });
  }

  function emitDocument(): void {
    const current = view.value;
    if (!current) return;
    if (documentStale) {
      syncedDocument = documentText(current.state);
      documentStale = false;
    }
    const selection = current.state.selection.main;
    options.updateDocument({
      value: syncedDocument,
      selectionStart: docPositionToSourceOffset(current.state, selection.from),
      selectionEnd: docPositionToSourceOffset(current.state, selection.to)
    });
  }

  function cancelScheduledSync(): void {
    if (syncTimer !== null) clearTimeout(syncTimer);
    syncTimer = null;
    syncPending = false;
  }

  function flushDocument(): void {
    cancelScheduledSync();
    emitDocument();
  }

  function scheduleSync(): void {
    if (syncTimer !== null) {
      syncPending = true;
      return;
    }
    emitDocument();
    syncTimer = setTimeout(() => {
      syncTimer = null;
      if (!syncPending) return;
      syncPending = false;
      scheduleSync();
    }, DOCUMENT_SYNC_INTERVAL_MS);
  }

  const updateListener = EditorView.updateListener.of((update) => {
    if (update.docChanged) {
      documentStale = true;
      scheduleSync();
    } else if (update.selectionSet) {
      emitDocument();
    }
  });

  const blurHandler = EditorView.domEventHandlers({
    blur: () => {
      flushDocument();
      return false;
    }
  });

  function extensions(): Extension[] {
    return [
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.lineWrapping,
      search(),
      sectionRangeField,
      sectionDecorations,
      activeQueryField,
      matchHighlighter,
      scrollMargins,
      editorTheme,
      updateListener,
      blurHandler,
      languageCompartment.of(languageExtension()),
      editableCompartment.of(editableExtension()),
      labelCompartment.of(labelExtension()),
      phraseCompartment.of(searchPhrases())
    ];
  }

  function initialState(text: string): EditorState {
    lineSeparator = lineSeparatorOf(text);
    const doc = Text.of(splitLines(text, lineSeparator));
    return EditorState.create({
      doc,
      selection: { anchor: doc.length },
      extensions: [extensions(), EditorState.lineSeparator.of(lineSeparator)]
    });
  }

  function documentText(state: EditorState): string {
    return state.doc.sliceString(0, state.doc.length, lineSeparator);
  }

  function docPositionToSourceOffset(state: EditorState, pos: number): number {
    const clamped = Math.max(0, Math.min(state.doc.length, pos));
    if (lineSeparator !== '\r\n') return clamped;
    return clamped + state.doc.lineAt(clamped).number - 1;
  }

  function sourceOffsetToDocPosition(state: EditorState, offset: number): number {
    const doc = state.doc;
    if (lineSeparator !== '\r\n') return Math.max(0, Math.min(doc.length, offset));
    const clamped = Math.max(0, Math.min(doc.length + doc.lines - 1, offset));
    let low = 1;
    let high = doc.lines;
    while (low < high) {
      const mid = Math.ceil((low + high) / 2);
      if (doc.line(mid).from + mid - 1 <= clamped) low = mid;
      else high = mid - 1;
    }
    const line = doc.line(low);
    return Math.min(clamped - (low - 1), line.to);
  }

  function createView(host: HTMLElement): void {
    syncedDocument = options.content();
    documentStale = false;
    view.value = new EditorView({
      parent: host,
      state: initialState(syncedDocument)
    });
    pushSectionRange();
    pushSearchQuery();
  }

  function destroyView(): void {
    cancelScheduledSync();
    view.value?.destroy();
    view.value = null;
  }

  function resetDocument(text: string): void {
    const current = view.value;
    if (!current) return;
    cancelScheduledSync();
    syncedDocument = text;
    documentStale = false;
    current.setState(initialState(text));
    pushSectionRange();
    pushSearchQuery();
  }

  function syncExternalValue(text: string): void {
    const current = view.value;
    if (!current) return;
    if (documentText(current.state) === text) {
      syncedDocument = text;
      documentStale = false;
      return;
    }
    if (lineSeparatorOf(text) !== lineSeparator) {
      resetDocument(text);
      return;
    }

    const previous = current.state.doc.toString();
    const next = splitLines(text, lineSeparator).join('\n');
    let start = 0;
    const shortest = Math.min(previous.length, next.length);
    while (start < shortest && previous.charCodeAt(start) === next.charCodeAt(start)) start += 1;
    let previousEnd = previous.length;
    let nextEnd = next.length;
    while (
      previousEnd > start &&
      nextEnd > start &&
      previous.charCodeAt(previousEnd - 1) === next.charCodeAt(nextEnd - 1)
    ) {
      previousEnd -= 1;
      nextEnd -= 1;
    }

    cancelScheduledSync();
    current.dispatch({
      changes: {
        from: start,
        to: previousEnd,
        insert: Text.of(next.slice(start, nextEnd).split('\n'))
      }
    });
    syncedDocument = text;
    documentStale = false;
    cancelScheduledSync();
  }

  function pushSectionRange(): void {
    const current = view.value;
    if (!current) return;
    const requested = options.viewRange();
    const mapped = requested
      ? {
          startOffset: sourceOffsetToDocPosition(current.state, requested.startOffset),
          endOffset: sourceOffsetToDocPosition(current.state, requested.endOffset)
        }
      : null;
    const stored = current.state.field(sectionRangeField);
    if (
      (mapped === null && stored === null) ||
      (mapped &&
        stored &&
        mapped.startOffset === stored.startOffset &&
        mapped.endOffset === stored.endOffset)
    ) return;
    current.dispatch({ effects: setSectionRange.of(mapped) });
  }

  function buildQuery(): SearchQuery {
    return new SearchQuery({
      search: findQuery.value,
      replace: replaceQuery.value,
      caseSensitive: caseSensitive.value,
      regexp: useRegexp.value,
      wholeWord: wholeWord.value,
      test: (_match, state, from, to) => {
        if (options.findScope() !== 'section') return true;
        const range = state.field(sectionRangeField, false);
        return !range || (from >= range.startOffset && to <= range.endOffset);
      }
    });
  }

  function pushSearchQuery(): boolean {
    const current = view.value;
    if (!current) return false;
    const query = buildQuery();
    current.dispatch({ effects: [setSearchQuery.of(query), setActiveQuery.of(query)] });
    return query.valid;
  }

  function matchStats(): { ordinal: number; total: number } {
    const current = view.value;
    const query = current ? queryOf(current.state) : null;
    if (!current || !query?.valid) return { ordinal: 0, total: 0 };

    const selection = current.state.selection.main;
    const cursor = query.getCursor(current.state);
    let total = 0;
    let ordinal = 0;
    for (let step = cursor.next(); !step.done; step = cursor.next()) {
      total += 1;
      if (step.value.from === selection.from && step.value.to === selection.to) ordinal = total;
    }
    return { ordinal, total };
  }

  function reportMatchStats(): { ordinal: number; total: number } {
    const stats = matchStats();
    findState.value = stats.ordinal > 0
      ? { kind: 'match', ordinal: stats.ordinal, total: stats.total }
      : { kind: 'count', total: stats.total };
    return stats;
  }

  function readyView(): EditorView | null {
    const current = view.value;
    if (!current || options.disabled() || !findQuery.value) return null;
    if (!pushSearchQuery()) {
      findState.value = { kind: 'invalidRegexp' };
      return null;
    }
    return current;
  }

  function runFind(backward: boolean): void {
    const current = readyView();
    if (!current) return;
    const moved = backward ? cmFindPrevious(current) : cmFindNext(current);
    if (!moved) {
      findState.value = { kind: 'noMatches' };
      return;
    }
    current.focus();
    reportMatchStats();
  }

  function findNext(): void {
    runFind(false);
  }

  function findPrevious(): void {
    runFind(true);
  }

  function selectionIsMatch(current: EditorView): boolean {
    const query = queryOf(current.state);
    const selection = current.state.selection.main;
    if (!query?.valid || selection.empty) return false;
    const cursor = query.getCursor(current.state, selection.from, selection.to);
    const step = cursor.next();
    return !step.done && step.value.from === selection.from && step.value.to === selection.to;
  }

  function replaceNext(): void {
    const current = readyView();
    if (!current) return;

    // CodeMirror search 6.7.2 replaceNext replaces only an exactly selected match; otherwise it selects the next match.
    if (!selectionIsMatch(current) && !cmFindNext(current)) {
      findState.value = { kind: 'noMatches' };
      return;
    }
    if (!cmReplaceNext(current)) {
      findState.value = { kind: 'noMatches' };
      return;
    }
    current.focus();

    const stats = matchStats();
    if (stats.total === 0) {
      findState.value = { kind: 'replacedOneNoneRemain' };
    } else if (stats.ordinal > 0) {
      findState.value = {
        kind: 'replacedOneThenMatch',
        ordinal: stats.ordinal,
        total: stats.total
      };
    } else {
      findState.value = { kind: 'replacedOneThenCount', total: stats.total };
    }
  }

  function onReplaceInputKeydown(event: KeyboardEvent): void {
    if (event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    replaceNext();
  }

  function replaceAll(): void {
    const current = readyView();
    if (!current) return;
    const { total } = matchStats();
    if (total === 0 || !cmReplaceAll(current)) {
      findState.value = { kind: 'noMatches' };
      return;
    }
    findState.value = { kind: 'replaced', count: total };
  }

  function getCurrentParagraphStart(): number {
    const current = view.value;
    if (!current) return 0;
    const start = paragraphStartOffset(
      current.state.doc.toString(),
      current.state.selection.main.from
    );
    return docPositionToSourceOffset(current.state, start);
  }

  function dispatchReplace(
    startOffset: number,
    endOffset: number,
    replacement: string,
    select: boolean
  ): void {
    const current = view.value;
    if (!current) return;
    const from = sourceOffsetToDocPosition(current.state, startOffset);
    const to = Math.max(from, sourceOffsetToDocPosition(current.state, endOffset));
    const insert = Text.of(replacement.split(/\r\n?|\n/));
    current.dispatch({
      changes: { from, to, insert },
      selection: select ? EditorSelection.range(from, from + insert.length) : undefined,
      scrollIntoView: select
    });
    if (select) current.focus();
    flushDocument();
  }

  function replaceRange(startOffset: number, endOffset: number, replacement: string): void {
    dispatchReplace(startOffset, endOffset, replacement, true);
  }

  function replaceRangeQuietly(
    startOffset: number,
    endOffset: number,
    replacement: string
  ): void {
    dispatchReplace(startOffset, endOffset, replacement, false);
  }

  function focusAndSelect(startOffset: number, endOffset: number): void {
    const current = view.value;
    if (!current) return;
    const from = sourceOffsetToDocPosition(current.state, startOffset);
    const to = Math.max(from, sourceOffsetToDocPosition(current.state, endOffset));
    const selection = EditorSelection.range(from, to);
    current.focus();
    current.dispatch({
      selection,
      effects: EditorView.scrollIntoView(selection, { y: 'start' })
    });
  }

  function jumpToOffset(offset: number): void {
    focusAndSelect(offset, offset);
  }

  watch(hostRef, (host) => {
    destroyView();
    if (host) createView(host);
  });

  watch(options.sourceId, () => {
    findQuery.value = '';
    replaceQuery.value = '';
    findState.value = null;
    resetDocument(options.content());
  });

  watch(options.content, (value) => {
    if (value === syncedDocument) return;
    syncExternalValue(value);
  });

  watch(options.disabled, () => {
    view.value?.dispatch({
      effects: editableCompartment.reconfigure(editableExtension())
    });
  });

  watch(options.format, () => {
    view.value?.dispatch({
      effects: languageCompartment.reconfigure(languageExtension())
    });
  });

  watch(options.contentLabel, () => {
    view.value?.dispatch({
      effects: [
        labelCompartment.reconfigure(labelExtension()),
        phraseCompartment.reconfigure(searchPhrases())
      ]
    });
  });

  watch(options.viewRange, pushSectionRange);

  watch([findQuery, replaceQuery, caseSensitive, useRegexp, wholeWord], () => {
    findState.value = null;
    if (findQuery.value) pushSearchQuery();
  });

  watch(options.findScope, () => {
    findState.value = null;
    if (findQuery.value) pushSearchQuery();
  });

  onBeforeUnmount(destroyView);

  return {
    hostRef,
    view,
    findQuery,
    replaceQuery,
    caseSensitive,
    useRegexp,
    wholeWord,
    findStatus,
    disableFind,
    findNext,
    findPrevious,
    replaceNext,
    onReplaceInputKeydown,
    replaceAll,
    flushDocument,
    getCurrentParagraphStart,
    replaceRange,
    replaceRangeQuietly,
    jumpToOffset,
    focusAndSelect
  };
}
