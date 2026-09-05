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

/**
 * The shortest gap between two document snapshots handed to the host.
 *
 * The page only needs a copy to derive the outline, the dirty flag and the save
 * payload, so a burst of typing publishes on its first change and then at most
 * this often, with a trailing snapshot once it stops. `flushDocument()` covers
 * the callers that need an exact, current document.
 */
const DOCUMENT_SYNC_INTERVAL_MS = 150;

interface SourceCodeMirrorOptions {
  content: () => string;
  sourceId: () => string;
  disabled: () => boolean;
  /** The focused chapter, or null while the whole source is shown. */
  viewRange: () => SourceEditorViewRange | null;
  findScope: () => SourceFindScope;
  /** Only a Markdown source is parsed as Markdown; a TXT one is prose. */
  format: () => 'txt' | 'md';
  contentLabel: () => string;
  updateDocument: (edit: SourceDocumentEdit) => void;
}

/**
 * What the find/replace line is currently reporting, as data.
 *
 * Not the rendered sentence: a translated string held in a ref freezes at
 * whichever locale produced it, so the status would stay in the old language
 * after a switch while the controls around it updated.
 */
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
    // English distinguishes one from many here, and did so before these
    // strings moved into the catalog; the catalog has no plural rules, so the
    // two forms are separate keys. Replace-next always replaces exactly one.
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

/**
 * CodeMirror stores line breaks as single positions and renders back with `\n`
 * unless told otherwise, so a source saved from a CRLF file would come back
 * rewritten end to end — a whole-file diff for a project whose files are the
 * source of truth. A consistently CRLF document therefore keeps it, both for
 * what the editor emits and for what pressing Enter inserts.
 *
 * A document that mixes endings has no separator that round-trips it, so those
 * normalize to `\n`.
 */
function lineSeparatorOf(text: string): string {
  if (!text.includes('\r')) return '\n';
  const mixed = /(?<!\r)\n/.test(text) || /\r(?!\n)/.test(text);
  return mixed ? '\n' : '\r\n';
}

/** Splits into lines the way the chosen separator implies. */
function splitLines(text: string, separator: string): string[] {
  return separator === '\r\n' ? text.split('\r\n') : text.split(/\r\n?|\n/);
}

/** Replaces the dimmed range; null shows the whole source undimmed. */
const setSectionRange = StateEffect.define<SourceEditorViewRange | null>();

/**
 * The focused chapter as a range the editor maps through every change, rather
 * than a slice: between two pushes from the page the boundary tracks the text
 * instead of drifting.
 */
const sectionRangeField = StateField.define<SourceEditorViewRange | null>({
  create: () => null,
  update(value, tr) {
    let next = value;
    for (const effect of tr.effects) {
      if (effect.is(setSectionRange)) next = effect.value;
    }
    if (!next) return null;
    if (!tr.docChanged) return next;
    // Both edges are greedy so that text typed against a boundary stays inside
    // the chapter the caret is in rather than falling into the dimmed part.
    const startOffset = tr.changes.mapPos(next.startOffset, -1);
    const endOffset = tr.changes.mapPos(next.endOffset, 1);
    return {
      startOffset: Math.min(startOffset, endOffset),
      endOffset: Math.max(startOffset, endOffset)
    };
  }
});

const dimmedMark = Decoration.mark({ class: 'cm-source-dimmed' });

/** Dims everything outside the focused chapter without touching the document. */
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

/**
 * The query our own find bar last installed.
 *
 * `@codemirror/search` only highlights matches while its own panel is open, and
 * this editor deliberately keeps the panel closed and drives the same commands
 * from the existing Vue controls. Holding the query separately lets the
 * highlighter below run without the panel, and keeps the highlighted set
 * identical to the one `findNext`/`replaceAll` act on.
 */
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

/** Extra context scanned around the viewport so matches crossing it still light up. */
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

/**
 * Markdown styled as prose rather than as code.
 *
 * The reader renders these sources with `html: true`, so the embedded tags are
 * part of the format and are highlighted alongside it. The syntax characters
 * themselves — hashes, bullets, fences, emphasis markers — recede instead of
 * being coloured, so what is left reading like the text it produces.
 */
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
  // Opacity, not colour: the syntax highlighting below paints colours onto
  // spans nested inside this one, and those would win. Opacity applies to the
  // whole subtree, so the chapter outside the focused one recedes either way.
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

/**
 * Keeps the space a match is scrolled into clear of the panel edges.
 *
 * Everything that reveals a position — the chapter jumps, the find commands,
 * the editor's own caret tracking — goes through `EditorView.scrollIntoView`,
 * which subtracts these margins from the visible box. Anything that later
 * overlays the editor (a sticky chapter header, for instance) belongs here too,
 * rather than in a per-call `yMargin` or a hand-computed `scrollTop`.
 */
const scrollMargins = EditorView.scrollMargins.of(() => ({ top: 24, bottom: 24 }));

/** The announcements `@codemirror/search` makes through its own live region. */
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
  // shallowRef, never ref()/reactive(): CodeMirror state is a large immutable
  // graph that a deep proxy would both walk and hand back wrapped, and a wrapped
  // EditorState fails CodeMirror's own identity checks.
  const view = shallowRef<EditorView | null>(null);

  const findQuery = ref('');
  const replaceQuery = ref('');
  const caseSensitive = ref(false);
  const useRegexp = ref(false);
  const wholeWord = ref(false);
  const findState = ref<FindState | null>(null);
  // Rendered here rather than in the component so the sentence re-resolves on a
  // locale change: t() reads the locale ref, and this computed depends on it.
  const findStatus = computed(() => renderFindState(findState.value));
  const disableFind = computed(() => options.disabled() || !findQuery.value);

  const languageCompartment = new Compartment();
  const editableCompartment = new Compartment();
  const labelCompartment = new Compartment();
  const phraseCompartment = new Compartment();

  /** The line ending the current source uses; see lineSeparatorOf. */
  let lineSeparator = '\n';
  /** The document the host has already been told about. */
  let syncedDocument = '';
  /** Set while the editor holds changes that `syncedDocument` predates. */
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
      selectionStart: docToStringOffset(current.state, selection.from),
      selectionEnd: docToStringOffset(current.state, selection.to)
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

  /**
   * Publishes now if the interval has elapsed, otherwise once it does.
   *
   * Publishing on the leading edge is what keeps "unsaved changes" and the Save
   * button honest from the first keystroke; the interval only limits how often
   * the rope is flattened while typing continues.
   */
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
      // Nothing was rewritten, so the cached document is still exact and the
      // caret can reach the chapter outline without waiting for the interval.
      emitDocument();
    }
  });

  const blurHandler = EditorView.domEventHandlers({
    blur: () => {
      // Buttons outside the editor act on the document the page holds, and a
      // click blurs the editor before it fires. Flushing here keeps those
      // handlers reading exactly what is on screen.
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

  /**
   * A freshly loaded source starts with the caret at its end.
   *
   * That is where a textarea leaves it after its value is assigned, and the
   * chapter outline's "Add" button inserts at the caret's paragraph — so
   * starting anywhere else would silently move new chapters from the end of an
   * untouched book to its beginning. The document is built first because a line
   * break is one position to CodeMirror however many characters it was written
   * with, so the text's length is not the document's.
   */
  function initialState(text: string): EditorState {
    lineSeparator = lineSeparatorOf(text);
    const doc = Text.of(splitLines(text, lineSeparator));
    return EditorState.create({
      doc,
      selection: { anchor: doc.length },
      extensions: [extensions(), EditorState.lineSeparator.of(lineSeparator)]
    });
  }

  /** The document as text, written back with the line ending it came in with. */
  function documentText(state: EditorState): string {
    return state.doc.sliceString(0, state.doc.length, lineSeparator);
  }

  /**
   * The same position, measured in the file's characters rather than the
   * document's.
   *
   * The page indexes `content` as a plain string, where a CRLF is two
   * characters, and derives every chapter range from it; CodeMirror counts a
   * line break as one position however it was written. Both directions of this
   * boundary are converted here — the caret the editor publishes, the focused
   * range it is handed, the ranges chapter edits act on — so that the whole
   * adapter contract is stated in the file's coordinates and nothing outside
   * this composable has to know the document has its own.
   *
   * A document that mixes endings normalizes to `\n` on the way in, so its text
   * and the file's diverge until the first edit publishes the normalized
   * version; identity is the closest either direction can get in that window.
   */
  function docToStringOffset(state: EditorState, pos: number): number {
    const clamped = Math.max(0, Math.min(state.doc.length, pos));
    if (lineSeparator !== '\r\n') return clamped;
    return clamped + state.doc.lineAt(clamped).number - 1;
  }

  /** The inverse: a position in the file's characters, as a document position. */
  function stringToDocOffset(state: EditorState, offset: number): number {
    const doc = state.doc;
    if (lineSeparator !== '\r\n') return Math.max(0, Math.min(doc.length, offset));
    // One character per line break precedes each line, so a line's own start is
    // the last one at or before the offset and the search below is monotonic.
    const clamped = Math.max(0, Math.min(doc.length + doc.lines - 1, offset));
    let low = 1;
    let high = doc.lines;
    while (low < high) {
      const mid = Math.ceil((low + high) / 2);
      if (doc.line(mid).from + mid - 1 <= clamped) low = mid;
      else high = mid - 1;
    }
    const line = doc.line(low);
    // An offset landing between a `\r` and its `\n` is the end of the line they
    // terminate together: the pair is one position in the document.
    return Math.min(clamped - (low - 1), line.to);
  }

  function createView(host: HTMLElement): void {
    // The host's own string, not the document's: a source that mixes line
    // endings normalizes on the way in, and reporting that as an unsaved change
    // before the user has touched anything would be a lie. The first real edit
    // publishes the normalized text, which is what the textarea did too.
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

  /** Replaces the whole document and drops the undo history with it. */
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

  /**
   * Applies an externally supplied document as the single range that differs.
   *
   * A full replacement would move every position in the document, which is
   * exactly what this migration removed; narrowing it to the changed span lets
   * CodeMirror map the selection, the chapter range and the undo history
   * through it like any other edit.
   */
  function syncExternalValue(text: string): void {
    const current = view.value;
    if (!current) return;
    if (documentText(current.state) === text) {
      syncedDocument = text;
      documentStale = false;
      return;
    }
    if (lineSeparatorOf(text) !== lineSeparator) {
      // Nothing can be mapped across a change of line ending, and a document
      // that arrives written differently is a reload rather than an edit.
      resetDocument(text);
      return;
    }

    // Diffed in document coordinates, where a line break is one position
    // whatever it was written with, so the offsets below are dispatchable.
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
    // The field holds document positions so that it maps through every edit;
    // what the page asks for is stated in the file's characters.
    const mapped = requested
      ? {
          startOffset: stringToDocOffset(current.state, requested.startOffset),
          endOffset: stringToDocOffset(current.state, requested.endOffset)
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
      // The chapter scope is a filter on matches, not a slice of the document:
      // findNext, replaceAll and the highlighter all consult it, so restricting
      // the search to the focused chapter costs nothing beyond this predicate.
      // It reads the mapped range out of the state so it stays correct while the
      // chapter's boundaries move under an edit.
      test: (_match, state, from, to) => {
        if (options.findScope() !== 'section') return true;
        const range = state.field(sectionRangeField, false);
        return !range || (from >= range.startOffset && to <= range.endOffset);
      }
    });
  }

  /** Installs the current query and reports whether it can be searched with. */
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

  /** Prepares the shared preconditions for every find/replace button. */
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

  /** True when the selection is exactly a match, which is what replaceNext replaces. */
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

    // CodeMirror's replaceNext only replaces a match the selection already
    // covers exactly, and otherwise just moves to it. Selecting first keeps the
    // button a one-click replace, which is what the control promised before.
    if (!selectionIsMatch(current) && !cmFindNext(current)) {
      findState.value = { kind: 'noMatches' };
      return;
    }
    if (!cmReplaceNext(current)) {
      findState.value = { kind: 'noMatches' };
      return;
    }
    current.focus();

    // replaceNext leaves the following match selected, so the counts below
    // describe what is left rather than what was replaced. Folding the two into
    // one sentence keeps the status translatable as a whole.
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
    // toString(), not documentText(): the caret is a document position, so the
    // text it is measured against has to use one character per line break too.
    // The result is handed back to the page, so it converts on the way out.
    const start = paragraphStartOffset(
      current.state.doc.toString(),
      current.state.selection.main.from
    );
    return docToStringOffset(current.state, start);
  }

  function dispatchReplace(
    startOffset: number,
    endOffset: number,
    replacement: string,
    select: boolean
  ): void {
    const current = view.value;
    if (!current) return;
    const from = stringToDocOffset(current.state, startOffset);
    const to = Math.max(from, stringToDocOffset(current.state, endOffset));
    // Built as a document rather than passed as a string: the caller writes its
    // own line breaks, and this is what both renders them with the document's
    // ending and makes the selection below a position rather than a character
    // count.
    const insert = Text.of(replacement.split(/\r\n?|\n/));
    current.dispatch({
      changes: { from, to, insert },
      selection: select ? EditorSelection.range(from, from + insert.length) : undefined,
      scrollIntoView: select
    });
    if (select) current.focus();
    // The caller computed this edit from the document it holds; publishing the
    // result immediately keeps a follow-up edit from being computed against the
    // pre-edit copy.
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
    const from = stringToDocOffset(current.state, startOffset);
    const to = Math.max(from, stringToDocOffset(current.state, endOffset));
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
