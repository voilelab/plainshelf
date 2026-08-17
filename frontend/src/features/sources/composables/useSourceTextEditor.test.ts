// @vitest-environment jsdom
import { nextTick, ref } from 'vue';
import { describe, expect, it, vi } from 'vitest';

import { useSourceTextEditor } from './useSourceTextEditor';
import type {
  SourceDocumentEdit,
  SourceEditorViewRange
} from '@/features/sources/types/editorAdapter';

interface Harness {
  textarea: HTMLTextAreaElement;
  edits: SourceDocumentEdit[];
  content: () => string;
  type: (text: string) => Promise<void>;
}

function mountEditor(
  initialContent: string,
  viewRange: SourceEditorViewRange | null = null
): Harness & ReturnType<typeof useSourceTextEditor> {
  const content = ref(initialContent);
  const range = ref(viewRange);
  const edits: SourceDocumentEdit[] = [];

  const textarea = document.createElement('textarea');
  textarea.value = range.value
    ? content.value.slice(range.value.startOffset, range.value.endOffset)
    : content.value;
  document.body.append(textarea);

  const editor = useSourceTextEditor({
    content: () => content.value,
    sourceId: () => 'source-1',
    disabled: () => false,
    viewRange: () => range.value,
    findScope: () => 'source',
    updateDocument: (edit) => {
      edits.push(edit);
      content.value = edit.value;
      // The page widens the projection by the edit's delta rather than
      // reprojecting immediately, which is what keeps the textarea's value
      // untouched while typing.
      if (range.value) {
        range.value = {
          ...range.value,
          endOffset: range.value.startOffset + textarea.value.length
        };
      }
    },
    requestViewOffset: () => {}
  });
  editor.textareaRef.value = textarea;

  /**
   * Types at the caret the way the browser does, then fires `input`. The caret
   * moves by assignment rather than setSelectionRange so that the tests can
   * treat any setSelectionRange call as the composable's own doing.
   */
  async function type(text: string): Promise<void> {
    const caret = textarea.selectionStart;
    textarea.value =
      textarea.value.slice(0, caret) + text + textarea.value.slice(textarea.selectionEnd);
    textarea.selectionStart = caret + text.length;
    textarea.selectionEnd = caret + text.length;
    editor.onInput({ target: textarea } as unknown as Event);
    await nextTick();
    await nextTick();
  }

  return { ...editor, textarea, edits, content: () => content.value, type };
}

describe('useSourceTextEditor', () => {
  it('leaves focus and selection alone while typing', async () => {
    const editor = mountEditor('alpha\nbravo\ncharlie\n');
    editor.textarea.selectionStart = 5;
    editor.textarea.selectionEnd = 5;

    const focus = vi.spyOn(editor.textarea, 'focus');
    const setSelectionRange = vi.spyOn(editor.textarea, 'setSelectionRange');

    await editor.type('X');

    // The document took the edit, but nothing re-asserted the caret: the
    // textarea already shows exactly what the browser produced, and touching it
    // again is what used to drag the scroll position around on every keystroke.
    expect(editor.content()).toBe('alphaX\nbravo\ncharlie\n');
    expect(focus).not.toHaveBeenCalled();
    expect(setSelectionRange).not.toHaveBeenCalled();
  });

  it('leaves the caret alone while typing inside a focused section', async () => {
    const content = '## One\n\nfirst\n\n## Two\n\nsecond\n';
    const startOffset = content.indexOf('## Two');
    const editor = mountEditor(content, {
      startOffset,
      endOffset: content.length,
      key: 1
    });
    editor.textarea.selectionStart = editor.textarea.value.length;
    editor.textarea.selectionEnd = editor.textarea.value.length;

    const setSelectionRange = vi.spyOn(editor.textarea, 'setSelectionRange');

    await editor.type('!');

    expect(editor.content()).toBe('## One\n\nfirst\n\n## Two\n\nsecond\n!');
    expect(editor.edits[editor.edits.length - 1]?.selectionEnd).toBe(content.length + 1);
    expect(setSelectionRange).not.toHaveBeenCalled();
  });

  it('restores the caret when the projection was rewritten underneath it', async () => {
    const editor = mountEditor('alpha\nbravo\ncharlie\n');
    editor.textarea.selectionStart = 5;
    editor.textarea.selectionEnd = 5;

    const caret = editor.textarea.selectionStart;
    editor.textarea.value = `${editor.textarea.value.slice(0, caret)}X${editor.textarea.value.slice(caret)}`;
    editor.textarea.selectionStart = caret + 1;
    editor.textarea.selectionEnd = caret + 1;
    editor.onInput({ target: editor.textarea } as unknown as Event);

    // Stand in for Vue patching `value`, which drops the selection to the end.
    editor.textarea.selectionStart = editor.textarea.value.length;
    editor.textarea.selectionEnd = editor.textarea.value.length;
    await nextTick();
    await nextTick();

    expect(editor.textarea.selectionStart).toBe(6);
    expect(editor.textarea.selectionEnd).toBe(6);
  });
});
