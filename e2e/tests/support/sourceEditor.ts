import { expect, type Locator, type Page } from '@playwright/test';

// CodeMirror view 6.43.13 renders document ranges around its viewport; findFromDOM resolves the content element through cmTile.root.view (dist/index.js).
const FIND_CODEMIRROR_VIEW = `(() => {
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

export function editorText(page: Page): Promise<string> {
  return page.evaluate(`${FIND_CODEMIRROR_VIEW}.state.doc.toString()`) as Promise<string>;
}

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
    [FIND_CODEMIRROR_VIEW, text]
  );
  await expect.poll(() => editorText(page)).toBe(text);
}

export function editorSelectionText(page: Page): Promise<string> {
  return page.evaluate(`(() => {
    const view = ${FIND_CODEMIRROR_VIEW};
    const range = view.state.selection.main;
    return view.state.doc.sliceString(range.from, range.to);
  })()`) as Promise<string>;
}

export function editorScrollTop(page: Page): Promise<number> {
  return page.evaluate(`${FIND_CODEMIRROR_VIEW}.scrollDOM.scrollTop`) as Promise<number>;
}

export async function scrollEditorTo(page: Page, top: number): Promise<void> {
  await page.evaluate(`${FIND_CODEMIRROR_VIEW}.scrollDOM.scrollTop = ${top}`);
}

export async function setEditorCaret(page: Page, offset: number): Promise<void> {
  await page.evaluate(`(() => {
    const view = ${FIND_CODEMIRROR_VIEW};
    view.focus();
    view.dispatch({ selection: { anchor: ${offset} } });
  })()`);
}

export function hasMarkdownStyling(page: Page): Promise<boolean> {
  return page.evaluate(`Array.from(
    document.querySelectorAll('.source-content-editor .cm-content span'),
    (node) => getComputedStyle(node).fontWeight
  ).some((weight) => Number(weight) >= 700)`) as Promise<boolean>;
}
