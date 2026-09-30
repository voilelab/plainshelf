import { expect, type Locator, type Page } from '@playwright/test';

/**
 * Finds the CodeMirror view behind the source editor, from inside the page.
 *
 * The editor is not a form control any more, so `inputValue`, `fill` and
 * `selectionStart` no longer apply: the document lives in CodeMirror's state
 * and only the lines near the viewport exist in the DOM. `EditorView.findFromDOM`
 * is the supported lookup, but a page evaluated by Playwright cannot import the
 * module, so this repeats what that helper does — read the view CodeMirror parks
 * on the content element. The property it uses has been renamed once
 * (`cmView` → `cmTile`), so both spellings are probed rather than pinning the
 * tests to one CodeMirror release.
 */
const FIND_VIEW = `(() => {
  const content = document.querySelector('.source-content-editor .cm-content');
  for (const key of ['cmView', 'cmTile']) {
    const node = content && content[key];
    const view = node && (node.view || (node.root && node.root.view));
    if (view) return view;
  }
  throw new Error('CodeMirror view not found in the source editor');
})()`;

export function sourceEditor(page: Page): Locator {
  return page.locator('.source-content-editor .cm-content');
}

/** The whole document, including the parts CodeMirror has not rendered. */
export function editorText(page: Page): Promise<string> {
  return page.evaluate(`${FIND_VIEW}.state.doc.toString()`) as Promise<string>;
}

/** Replaces the document in one transaction, the way a paste would. */
export async function setEditorText(page: Page, text: string): Promise<void> {
  await expect(sourceEditor(page)).toBeVisible();
  await page.evaluate(
    ([findView, value]) => {
      const view = (0, eval)(findView) as {
        state: { doc: { length: number } };
        dispatch: (spec: unknown) => void;
        focus: () => void;
      };
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: value },
        selection: { anchor: 0 }
      });
      view.focus();
    },
    [FIND_VIEW, text]
  );
  await expect.poll(() => editorText(page)).toBe(text);
}

/** The text the editor currently has selected. */
export function editorSelectionText(page: Page): Promise<string> {
  return page.evaluate(`(() => {
    const view = ${FIND_VIEW};
    const range = view.state.selection.main;
    return view.state.doc.sliceString(range.from, range.to);
  })()`) as Promise<string>;
}

export function editorScrollTop(page: Page): Promise<number> {
  return page.evaluate(`${FIND_VIEW}.scrollDOM.scrollTop`) as Promise<number>;
}

export async function scrollEditorTo(page: Page, top: number): Promise<void> {
  await page.evaluate(`${FIND_VIEW}.scrollDOM.scrollTop = ${top}`);
}

/** Puts the caret at a document offset without going through find. */
export async function setEditorCaret(page: Page, offset: number): Promise<void> {
  await page.evaluate(`(() => {
    const view = ${FIND_VIEW};
    view.focus();
    view.dispatch({ selection: { anchor: ${offset} } });
  })()`);
}

/**
 * Whether any rendered line carries Markdown styling, i.e. the source is being
 * parsed as Markdown rather than shown as prose.
 */
export function hasMarkdownStyling(page: Page): Promise<boolean> {
  return page.evaluate(`Array.from(
    document.querySelectorAll('.source-content-editor .cm-content span'),
    (node) => getComputedStyle(node).fontWeight
  ).some((weight) => Number(weight) >= 700)`) as Promise<boolean>;
}
