// @vitest-environment jsdom
import { createApp, h, nextTick, ref, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { EditorSelection, type EditorState } from '@codemirror/state';
import { ensureSyntaxTree } from '@codemirror/language';

import { useSourceCodeMirror } from './useSourceCodeMirror';
import { insertChapterEdit } from '@/features/sources/utils/chapterEdits';
import type {
  SourceDocumentEdit,
  SourceEditorViewRange,
  SourceFindScope
} from '@/features/sources/types/editorAdapter';

type Editor = ReturnType<typeof useSourceCodeMirror>;

interface Harness {
  editor: Editor;
  app: App;
  edits: SourceDocumentEdit[];
  content: () => string;
  doc: () => string;
  lineBreak: () => string;
  state: () => EditorState;
  setFormat: (format: 'txt' | 'md') => Promise<void>;
  selection: () => { from: number; to: number };
  setViewRange: (range: SourceEditorViewRange | null) => Promise<void>;
  dimmed: () => string[];
  setFindScope: (scope: SourceFindScope) => Promise<void>;
  type: (from: number, to: number, insert: string) => void;
}

/**
 * Mounts the composable in a real component so that the editor is created,
 * torn down and driven exactly the way SourceEditor.vue drives it.
 */
async function mountEditor(initialContent: string): Promise<Harness> {
  const content = ref(initialContent);
  const viewRange = ref<SourceEditorViewRange | null>(null);
  const findScope = ref<SourceFindScope>('source');
  const format = ref<'txt' | 'md'>('md');
  const edits: SourceDocumentEdit[] = [];
  let editor!: Editor;

  const app = createApp({
    setup() {
      editor = useSourceCodeMirror({
        content: () => content.value,
        sourceId: () => 'source-1',
        disabled: () => false,
        viewRange: () => viewRange.value,
        findScope: () => findScope.value,
        format: () => format.value,
        contentLabel: () => 'Source content',
        updateDocument: (edit) => {
          edits.push(edit);
          content.value = edit.value;
        }
      });
      return () => h('div', { ref: editor.hostRef });
    }
  });
  app.mount(document.body.appendChild(document.createElement('div')));
  // The host element only exists after the first render, and the composable
  // creates the editor from a watcher on that ref.
  await nextTick();

  function view() {
    const current = editor.view.value;
    if (!current) throw new Error('editor view was not created');
    return current;
  }

  return {
    editor,
    app,
    edits,
    content: () => content.value,
    doc: () => view().state.doc.toString(),
    lineBreak: () => view().state.lineBreak,
    state: () => view().state,
    setFormat: async (next) => {
      format.value = next;
      await nextTick();
    },
    selection: () => {
      const range = view().state.selection.main;
      return { from: range.from, to: range.to };
    },
    setViewRange: async (range) => {
      viewRange.value = range;
      await nextTick();
    },
    // One entry per rendered line the dimming covers: a mark decoration is
    // split at every line, so the last entry is where the focused chapter
    // begins.
    dimmed: () =>
      Array.from(view().dom.querySelectorAll('.cm-source-dimmed')).map(
        (node) => node.textContent ?? ''
      ),
    setFindScope: async (scope) => {
      findScope.value = scope;
      await nextTick();
    },
    type: (from, to, insert) => {
      view().dispatch({
        changes: { from, to, insert },
        selection: EditorSelection.cursor(from + insert.length)
      });
    }
  };
}

const CHAPTERS = [
  '# Book',
  'Opening marker.',
  '',
  '## One',
  'First marker.',
  '',
  '## Two',
  'Second marker.',
  ''
].join('\n');

/** The range the page would report for the "## Two" chapter. */
function chapterTwoRange(): SourceEditorViewRange {
  return { startOffset: CHAPTERS.indexOf('## Two'), endOffset: CHAPTERS.length };
}

let harnesses: Harness[] = [];

async function mount(content: string): Promise<Harness> {
  const harness = await mountEditor(content);
  harnesses.push(harness);
  return harness;
}

beforeEach(() => {
  vi.useFakeTimers();
  harnesses = [];
});

afterEach(() => {
  for (const harness of harnesses) harness.app.unmount();
  vi.useRealTimers();
  document.body.innerHTML = '';
});

describe('useSourceCodeMirror document sync', () => {
  it('publishes the first change immediately and coalesces the rest', async () => {
    const editor = await mount('alpha\n');

    editor.type(5, 5, 'X');
    expect(editor.content()).toBe('alphaX\n');
    expect(editor.edits).toHaveLength(1);

    editor.type(6, 6, 'Y');
    editor.type(7, 7, 'Z');
    // Still one snapshot: the burst is inside the publish interval.
    expect(editor.edits).toHaveLength(1);
    expect(editor.content()).toBe('alphaX\n');

    vi.advanceTimersByTime(200);
    expect(editor.edits).toHaveLength(2);
    expect(editor.content()).toBe('alphaXYZ\n');
  });

  it('publishes the exact document when a caller flushes', async () => {
    const editor = await mount('alpha\n');

    editor.type(5, 5, 'X');
    editor.type(6, 6, 'Y');
    expect(editor.content()).toBe('alphaX\n');

    editor.editor.flushDocument();
    expect(editor.content()).toBe('alphaXY\n');
    // The pending trailing snapshot was consumed, not left to fire again.
    const published = editor.edits.length;
    vi.advanceTimersByTime(500);
    expect(editor.edits).toHaveLength(published);
  });

  it('does not reapply the document it just published', async () => {
    const editor = await mount('alpha\n');
    editor.type(5, 5, 'X');
    editor.editor.flushDocument();

    const before = editor.edits.length;
    vi.advanceTimersByTime(500);
    expect(editor.doc()).toBe('alphaX\n');
    expect(editor.edits).toHaveLength(before);
  });
});

describe('useSourceCodeMirror find and replace', () => {
  it('restricts find and replace to the focused chapter', async () => {
    const editor = await mount(CHAPTERS);
    await editor.setViewRange(chapterTwoRange());
    await editor.setFindScope('section');

    editor.editor.findQuery.value = 'marker';
    await nextTick();
    editor.editor.findNext();

    const selection = editor.selection();
    expect(selection.from).toBeGreaterThanOrEqual(chapterTwoRange().startOffset);
    expect(editor.doc().slice(selection.from, selection.to)).toBe('marker');
    // Three "marker"s exist in the source; only the chapter's one is counted.
    expect(editor.editor.findStatus.value).toContain('1');

    editor.editor.replaceQuery.value = 'flag';
    await nextTick();
    editor.editor.replaceAll();
    expect(editor.doc()).toContain('Opening marker.');
    expect(editor.doc()).toContain('First marker.');
    expect(editor.doc()).toContain('Second flag.');
  });

  it('searches the whole source once the scope is widened', async () => {
    const editor = await mount(CHAPTERS);
    await editor.setViewRange(chapterTwoRange());
    await editor.setFindScope('section');

    editor.editor.findQuery.value = 'Opening';
    await nextTick();
    const parked = editor.selection();
    editor.editor.findNext();
    // The one match lies outside the chapter, so nothing moved.
    expect(editor.editor.findStatus.value).toBe('No matches.');
    expect(editor.selection()).toEqual(parked);

    await editor.setFindScope('source');
    editor.editor.findNext();
    const selection = editor.selection();
    expect(editor.doc().slice(selection.from, selection.to)).toBe('Opening');
  });

  it('follows the chapter boundary through an edit made above it', async () => {
    const editor = await mount(CHAPTERS);
    await editor.setViewRange(chapterTwoRange());
    await editor.setFindScope('section');

    // Insert into the opening, which pushes the whole chapter down. Nothing
    // recomputes the boundary here; CodeMirror maps it through the change.
    editor.type(0, 0, 'PREFIX\n');

    editor.editor.findQuery.value = 'marker';
    await nextTick();
    editor.editor.findNext();
    const selection = editor.selection();
    expect(editor.doc().slice(selection.from - 7, selection.to)).toBe('Second marker');
  });

  it('replaces on the first click when nothing is selected yet', async () => {
    const editor = await mount('one two one two\n');
    editor.editor.findQuery.value = 'two';
    editor.editor.replaceQuery.value = 'three';
    await nextTick();

    editor.editor.replaceNext();
    expect(editor.doc()).toBe('one three one two\n');
    editor.editor.replaceNext();
    expect(editor.doc()).toBe('one three one three\n');
  });

  it('honours the case, whole-word and regular-expression options', async () => {
    const editor = await mount('Cat cats CAT\n');

    editor.editor.findQuery.value = 'cat';
    editor.editor.caseSensitive.value = true;
    await nextTick();
    editor.editor.findNext();
    expect(editor.selection()).toEqual({ from: 4, to: 7 });

    editor.editor.caseSensitive.value = false;
    editor.editor.wholeWord.value = true;
    await nextTick();
    editor.editor.replaceQuery.value = 'dog';
    editor.editor.replaceAll();
    expect(editor.doc()).toBe('dog cats dog\n');

    editor.editor.wholeWord.value = false;
    editor.editor.useRegexp.value = true;
    editor.editor.findQuery.value = 'd(o)g';
    editor.editor.replaceQuery.value = 'b$1x';
    await nextTick();
    editor.editor.replaceAll();
    expect(editor.doc()).toBe('box cats box\n');
  });

  it('reports an invalid regular expression instead of searching', async () => {
    const editor = await mount('alpha\n');
    editor.editor.useRegexp.value = true;
    editor.editor.findQuery.value = '(unclosed';
    await nextTick();

    const parked = editor.selection();
    editor.editor.findNext();
    expect(editor.editor.findStatus.value).toBe('That regular expression is not valid.');
    expect(editor.selection()).toEqual(parked);
  });
});

describe('useSourceCodeMirror editing adapter', () => {
  it('selects the replacement and publishes it at once', async () => {
    const editor = await mount('alpha bravo\n');

    editor.editor.replaceRange(6, 11, 'charlie');
    expect(editor.doc()).toBe('alpha charlie\n');
    expect(editor.selection()).toEqual({ from: 6, to: 13 });
    expect(editor.content()).toBe('alpha charlie\n');
  });

  it('leaves the caret alone for a quiet replacement', async () => {
    const editor = await mount('alpha bravo\n');
    editor.editor.focusAndSelect(3, 3);

    editor.editor.replaceRangeQuietly(6, 11, 'x');
    expect(editor.doc()).toBe('alpha x\n');
    expect(editor.selection()).toEqual({ from: 3, to: 3 });
  });

  it('starts a freshly loaded source with the caret at its end', async () => {
    const editor = await mount('alpha\nbravo\n');
    expect(editor.selection()).toEqual({ from: 12, to: 12 });
  });

  it('keeps a CRLF source written with CRLF', async () => {
    const editor = await mount('alpha\r\nbravo\r\n');

    editor.editor.replaceRange(5, 5, ' one');
    expect(editor.content()).toBe('alpha one\r\nbravo\r\n');
    // And a line break the user adds is written the same way.
    expect(editor.lineBreak()).toBe('\r\n');
  });

  it('normalizes a source that mixes line endings', async () => {
    const editor = await mount('alpha\r\nbravo\ncharlie\r\n');
    editor.editor.replaceRange(5, 5, '!');
    expect(editor.content()).toBe('alpha!\nbravo\ncharlie\n');
  });

  it('finds the start of the paragraph the caret sits in', async () => {
    const content = 'first\n\nsecond paragraph\n\nthird\n';
    const editor = await mount(content);

    editor.editor.focusAndSelect(content.indexOf('paragraph'), content.indexOf('paragraph'));
    expect(editor.editor.getCurrentParagraphStart()).toBe(content.indexOf('second'));
  });

  it('parses a Markdown source, and stops when the source is plain text', async () => {
    const editor = await mount('# Title\n\n**bold** text\n');
    const parsed = () =>
      ensureSyntaxTree(editor.state(), editor.state().doc.length)?.topNode.firstChild?.name ?? null;

    expect(parsed()).toBe('ATXHeading1');

    // A TXT source is prose, not Markdown; nothing in it should be styled as
    // syntax just because it happens to start with a hash.
    await editor.setFormat('txt');
    expect(parsed()).toBeNull();
  });

  it('destroys the view when the component unmounts', async () => {
    const editor = await mount('alpha\n');
    const view = editor.editor.view.value;
    expect(view).not.toBeNull();

    editor.app.unmount();
    harnesses = harnesses.filter((entry) => entry !== editor);
    expect(editor.editor.view.value).toBeNull();
    expect(view?.dom.isConnected).toBe(false);
  });
});
/**
 * The coordinates the page and the editor disagreed on.
 *
 * Everything the adapter takes or publishes is an offset into the source text,
 * where a CRLF is two characters; the document counts a line break as one
 * position. The drift is one character per preceding line break, so each case
 * below deliberately acts past several of them.
 */
describe('useSourceCodeMirror CRLF coordinates', () => {
  const CRLF_CHAPTERS = CHAPTERS.replace(/\n/g, '\r\n');

  it('dims up to the chapter heading, not past it', async () => {
    const editor = await mount(CRLF_CHAPTERS);
    await editor.setViewRange({
      startOffset: CRLF_CHAPTERS.indexOf('## Two'),
      endOffset: CRLF_CHAPTERS.length
    });

    const dimmed = editor.dimmed();
    // Six line breaks precede the heading, so reading the range as document
    // positions used to swallow "## Two" whole.
    expect(dimmed.join('')).not.toContain('Two');
    expect(dimmed[dimmed.length - 1]).toBe('First marker.');
    expect(dimmed[0]).toBe('# Book');
  });

  it('publishes the caret as an offset into the source text', async () => {
    const editor = await mount('alpha\r\nbravo\r\n');

    // Typed at a document position, the way the user's keystroke arrives.
    editor.type(6, 6, 'X');
    const published = editor.edits[editor.edits.length - 1];
    expect(published.value).toBe('alpha\r\nXbravo\r\n');
    expect(published.selectionStart).toBe(8);
    expect(published.value.slice(0, published.selectionStart)).toBe('alpha\r\nX');
  });

  it('replaces the range the source text names', async () => {
    const content = 'alpha\r\nbravo\r\ncharlie\r\n';
    const editor = await mount(content);
    const start = content.indexOf('bravo');

    editor.editor.replaceRange(start, start + 'bravo'.length, 'BRAVO');
    expect(editor.content()).toBe('alpha\r\nBRAVO\r\ncharlie\r\n');
  });

  it('reports a paragraph start a chapter insertion can be computed from', async () => {
    const content = 'first\r\n\r\nsecond paragraph\r\n\r\nthird\r\n';
    const editor = await mount(content);
    const caret = content.indexOf('paragraph');

    editor.editor.focusAndSelect(caret, caret);
    const start = editor.editor.getCurrentParagraphStart();
    expect(start).toBe(content.indexOf('second'));
    // The blank line before the paragraph is already there, so the heading is
    // inserted without one — which is only decidable in source coordinates.
    expect(insertChapterEdit(content, start).replacement).toBe(
      '## Untitled chapter\r\n\r\n'
    );
  });
});
