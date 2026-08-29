import { expect, test, type Page } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath } from './support/books';
import { openReaderTab } from './support/reader';
import {
  dimmedRanges,
  hasMarkdownStyling,
  editorScrollTop,
  editorSelectionText,
  editorText,
  scrollEditorTo,
  setEditorCaret,
  setEditorText,
  sourceEditor
} from './support/sourceEditor';

const getServer = useServer();

async function expectSourceEditorFitsViewport(page: Page): Promise<void> {
  const metrics = await page.evaluate(() => {
    const rect = (selector: string) => {
      const element = document.querySelector(selector);
      if (!(element instanceof HTMLElement)) {
        throw new Error(`Missing ${selector}`);
      }
      const bounds = element.getBoundingClientRect();
      return { top: bounds.top, bottom: bounds.bottom, height: bounds.height };
    };

    return {
      viewportHeight: window.innerHeight,
      page: rect('.source-editor-page'),
      editor: rect('.editor-panel'),
      surface: rect('.source-content-editor')
    };
  });

  expect(metrics.page.bottom).toBeLessThanOrEqual(metrics.viewportHeight + 1);
  expect(metrics.editor.bottom).toBeLessThanOrEqual(metrics.page.bottom + 1);
  expect(metrics.surface.top).toBeGreaterThanOrEqual(metrics.editor.top - 1);
  expect(metrics.surface.bottom).toBeLessThanOrEqual(metrics.editor.bottom + 1);
}

// Imports a plain-text book under a per-test title and opens its source editor.
// The shelf is shared across this file's tests, so each test passes its own
// unique name to avoid colliding with books left behind by earlier tests.
async function openSourceEditor(page: Page, baseUrl: string, title: string): Promise<void> {
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, title);
  await page.locator('.book-list-row').getByRole('heading', { name: title, exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Manage sources' }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+\/sources$/);
  await expect(page.getByText('No pending changes').first()).toBeVisible();
  await expect(sourceEditor(page)).toBeVisible();
}

test('should edit source content and see the change reflected in the reader', async ({ page }) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-edit');

  const editor = sourceEditor(page);
  await expect(editor).toBeVisible();
  await expectSourceEditorFitsViewport(page);
  // The imported source is TXT, so nothing in it is Markdown syntax.
  expect(await hasMarkdownStyling(page)).toBe(false);

  await page.setViewportSize({ width: 800, height: 500 });
  await expectSourceEditorFitsViewport(page);
  await page.setViewportSize({ width: 1280, height: 720 });
  await expectSourceEditorFitsViewport(page);

  // Append unique content to the existing source
  const original = await editorText(page);
  const filler = Array.from(
    { length: 40 },
    (_, index) => `Filler line ${index + 1}: ${'wrapped content '.repeat(24)}`
  ).join('\n');
  await setEditorText(
    page,
    `${original}\nrepeat repeat\n${filler}\nDeep search marker.\nEdited by E2E source editor.`
  );

  const findReplace = page.getByRole('group', { name: 'Find and replace' });
  await findReplace.getByLabel('Find').fill('PlainShelf');
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 1.');
  await expect.poll(() => editorSelectionText(page)).toBe('PlainShelf');

  await findReplace.getByLabel('Find').fill('Deep search marker');
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect.poll(() => editorSelectionText(page)).toBe('Deep search marker');
  // The match is thousands of pixels down: these filler lines wrap into
  // several visual rows each, and the height map accounts for every one of
  // them without the document ever being measured off-screen.
  await expect.poll(() => editorScrollTop(page)).toBeGreaterThan(2_000);

  await setEditorCaret(page, 0);
  await scrollEditorTo(page, 0);

  // Replace must find and replace in one click even when nothing is
  // currently selected; it must not require a separate Find click first.
  await findReplace.getByLabel('Find').fill('E2E');
  await findReplace.getByLabel('Replace').fill('browser');
  await findReplace.getByRole('button', { name: 'Replace', exact: true }).click();
  await expect.poll(() => editorText(page)).toMatch(/Hello from PlainShelf browser\./);
  await expect(findReplace.getByRole('status')).toHaveText('Replaced 1 occurrence. Match 1 of 1.');

  await findReplace.getByLabel('Find').fill('repeat');
  await findReplace.getByLabel('Replace').fill('done');
  await findReplace.getByRole('button', { name: 'Replace all' }).click();
  await expect.poll(() => editorText(page)).toMatch(/done done/);
  await expect(findReplace.getByRole('status')).toHaveText('Replaced 2 occurrences.');

  // Status should flip to "Unsaved changes" and Save button should enable
  await expect(page.getByText('Unsaved changes').first()).toBeVisible();
  const saveButton = page.getByRole('button', { name: 'Save*' });
  await expect(saveButton).toBeEnabled();
  await saveButton.click();

  // After save the topbar shows "Source saved."
  await expect(page.getByText('Source saved.')).toBeVisible();

  // Go back to the detail page, then open the reader (a new tab on the web build)
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: /reading/i }).click()
  );

  // Reader should display the appended line. It sits at the very bottom of a
  // long section, which the reader now virtualizes, so the block only mounts
  // once it is scrolled near the viewport. Scroll to the end, then assert.
  const readerContent = reader.locator('.reader-content');
  await expect(readerContent).toBeVisible();
  await expect(async () => {
    await readerContent.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
    await expect(reader.getByText('Edited by E2E source editor.')).toBeVisible({ timeout: 1000 });
  }).toPass();
});

test('should search with case, whole-word and regular-expression options', async ({ page }) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-search');
  await setEditorText(page, 'Cat cats CAT concat\n');

  const findReplace = page.getByRole('group', { name: 'Find and replace' });
  await findReplace.getByLabel('Find').fill('cat');
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 4.');
  await expect.poll(() => editorSelectionText(page)).toBe('Cat');

  // The caret is parked at the first match, so each option below is measured
  // from the same starting point.
  await findReplace.getByLabel('Match case').check();
  await setEditorCaret(page, 0);
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 2.');
  await expect.poll(() => editorSelectionText(page)).toBe('cat');

  await findReplace.getByLabel('Match case').uncheck();
  // "cats" and "concat" carry the word further, so only the two standalone
  // spellings count once whole-word matching is on.
  await findReplace.getByLabel('Whole word').check();
  await setEditorCaret(page, 0);
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 2.');
  await expect.poll(() => editorSelectionText(page)).toBe('Cat');

  await findReplace.getByLabel('Whole word').uncheck();
  await findReplace.getByLabel('Regular expression').check();
  await findReplace.getByLabel('Find').fill('c(a)ts?');
  await findReplace.getByLabel('Replace').fill('d[$1]');
  await findReplace.getByRole('button', { name: 'Replace all' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Replaced 4 occurrences.');
  // The capture keeps the text as it was written, so CAT yields d[A].
  await expect.poll(() => editorText(page)).toBe('d[a] d[a] d[A] cond[a]\n');

  await findReplace.getByLabel('Find').fill('d[a');
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('That regular expression is not valid.');
});

test('should create a new source, set it as current, and see its content in the reader', async ({
  page
}) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-newsource');

  // Create a new source
  const newBtn = page.getByRole('button', { name: 'New' });
  await newBtn.click();

  // Wait for the entire creation cycle to settle:
  //   - "New" button re-enabled means creating=false and loadSource finished
  //   - an empty, editable document confirms the new source is active
  //   - "No pending changes" confirms isDirty=false (content===initialContent==='')
  await expect(newBtn).toBeEnabled();
  const editor = sourceEditor(page);
  await expect(editor).toBeVisible();
  await expect.poll(() => editorText(page)).toBe('');
  await expect(page.getByText('No pending changes').first()).toBeVisible();

  await editor.click();
  await page.keyboard.type('This is the second source.');
  await expect.poll(() => editorText(page)).toBe('This is the second source.');

  // Save the new source
  await expect(page.getByText('Unsaved changes').first()).toBeVisible();
  const saveButton = page.getByRole('button', { name: 'Save*' });
  await expect(saveButton).toBeEnabled();
  await saveButton.click();
  await expect(page.getByText('Source saved.')).toBeVisible();

  // The server may or may not auto-set a newly created source as current.
  // Click "Set as current" only if it is present; otherwise proceed directly.
  if (await page.getByRole('button', { name: 'Set as current' }).isVisible()) {
    await page.getByRole('button', { name: 'Set as current' }).click();
    await expect(page.getByText('Current source updated.')).toBeVisible();
  }

  // Open the reader and verify the new source is rendered (new tab on web)
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );

  await expect(reader.getByText('This is the second source.')).toBeVisible();
});

test('should keep the book readable after deleting its current source', async ({ page }) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-delete');

  // The imported source is the one to keep; give the book a second source and
  // make that the current one, so deleting it exercises the hand-over.
  const importedId = await page.locator('.source-item .source-id').first().innerText();
  const newBtn = page.getByRole('button', { name: 'New' });
  await newBtn.click();
  await expect(newBtn).toBeEnabled();
  await expect(page.locator('.source-item')).toHaveCount(2);

  const doomedId = (await page.locator('.source-item .source-id').allInnerTexts())
    .find((id) => id !== importedId);
  if (!doomedId) {
    throw new Error(`Expected a second source alongside ${importedId}`);
  }

  const setCurrent = page.getByRole('button', { name: 'Set as current' });
  if (await setCurrent.isVisible()) {
    await setCurrent.click();
    await expect(page.getByText('Current source updated.')).toBeVisible();
  }
  await expect(page.locator('.source-item', { hasText: doomedId }).getByText('Current')).toBeVisible();

  // Deleting the current source must hand the pointer to the survivor rather
  // than leave the book pointing at something that no longer exists.
  await page.getByRole('button', { name: `Delete source ${doomedId}` }).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(page.locator('.source-item')).toHaveCount(1);
  await expect(page.locator('.source-item', { hasText: importedId }).getByText('Current')).toBeVisible();

  // The detail page and the reader both have to survive that.
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await expect(page.getByRole('heading', { name: 'srceditor-delete' })).toBeVisible();
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );
  await expect(reader.getByText('Hello from PlainShelf E2E.')).toBeVisible();
});

test('should derive chapterized Markdown from TXT and keep the original source', async ({ page }) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-derive');

  await page.getByRole('button', { name: 'Manual TXT → MD' }).click();
  const conversionDialog = page.getByRole('dialog', { name: 'Create chapterized Markdown source' });
  await expect(conversionDialog).toBeVisible();
  await expect(conversionDialog.getByText('Preview', { exact: true })).toBeVisible();
  await expect(conversionDialog.getByRole('checkbox', { name: 'Set the new source as current' })).toBeChecked();
  await conversionDialog.getByRole('button', { name: 'Create source' }).click();
  await expect(page.getByText('Derived source created.')).toBeVisible();
  await expect(page.getByText('2 total')).toBeVisible();
  await expect(page.getByText('MD', { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'Add', exact: true }).click();
  await expect.poll(() => editorText(page)).toMatch(/## Untitled chapter/);
  await page.getByRole('button', { name: 'Save*' }).click();
  await expect(page.getByText('Source saved.')).toBeVisible();

  await page.getByRole('button', { name: 'Back' }).click();
  // The reader opens in a new tab; the original tab stays on the detail page.
  const derivedReader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );
  await expect(derivedReader.getByText('1 / 2')).toBeVisible();
  await derivedReader.getByRole('button', { name: 'Next' }).click();
  await expect(derivedReader.getByRole('heading', { name: 'Untitled chapter' })).toBeVisible();
  await expect(derivedReader.getByText('This text came from a real uploaded TXT file.')).toBeVisible();
  await derivedReader.close();

  // Continue on the original detail tab instead of the reader's own back link.
  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Manage sources' }).click();
  const originalSource = page.locator('.source-item').filter({ hasNotText: 'Current' });
  await originalSource.locator('.source-item-content').click();
  await page.getByRole('button', { name: 'Set as current' }).click();
  await expect(page.getByText('Current source updated.')).toBeVisible();

  await page.getByRole('button', { name: 'Back' }).click();
  const originalReader = await openReaderTab(page, () =>
    page.getByRole('button', { name: /reading/i }).click()
  );
  await expect(originalReader.getByText('1 / 1')).toBeVisible();
  await expect(originalReader.getByRole('heading', { name: 'Untitled chapter' })).toHaveCount(0);
  await expect(originalReader.getByText('This text came from a real uploaded TXT file.')).toBeVisible();
});

test('should focus one Markdown chapter without splitting the document', async ({ page }) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-focus');

  await page.getByRole('button', { name: 'Manual TXT → MD' }).click();
  const conversionDialog = page.getByRole('dialog', { name: 'Create chapterized Markdown source' });
  await conversionDialog.getByRole('button', { name: 'Create source' }).click();
  await expect(page.getByText('Derived source created.')).toBeVisible();

  const original = [
    '# Focused book',
    'Opening marker.',
    '',
    '## One',
    'First marker repeat.',
    '',
    '## Two',
    'Second marker repeat.'
  ].join('\n');
  await setEditorText(page, original);

  await page.setViewportSize({ width: 700, height: 700 });
  await page.getByRole('button', { name: 'Chapters', exact: true }).click();
  await page.locator('.chapter-jump').filter({ hasText: 'One' }).click();
  await expect(sourceEditor(page)).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 720 });

  // A Markdown source is highlighted as Markdown: the headings above render
  // bold even though the source itself is unchanged.
  await expect.poll(() => hasMarkdownStyling(page)).toBe(true);

  // Focusing a chapter is now purely visual: the document is untouched and
  // only the text outside the chapter is dimmed.
  await expect.poll(() => editorText(page)).toBe(original);
  await expect.poll(async () => (await dimmedRanges(page)).join('\n')).toContain('Opening marker.');
  await expect.poll(async () => (await dimmedRanges(page)).join('\n')).toContain('Second marker repeat.');
  expect((await dimmedRanges(page)).join('\n')).not.toContain('First marker repeat.');
  await expect(page.getByLabel('Scope')).toHaveText('Current chapter');

  // Typing survives a chapter switch, and so does the undo history: neither
  // rebuilds the editor any more.
  //
  // The caret is placed by document offset rather than walked there with
  // ArrowDown/End. `End` runs CodeMirror's cursorLineBoundaryForward, which
  // stops at the end of the *visual* row, and this editor pane is a few hundred
  // pixels wide with line wrapping on — so whether the whole line fits in one
  // row comes down to the platform's font metrics. It does on CI and does not
  // on macOS, where the caret used to land mid-line and the assertion below
  // measured nothing.
  const firstChapterLine = 'First marker repeat.';
  await setEditorCaret(page, original.indexOf(firstChapterLine) + firstChapterLine.length);
  await page.keyboard.type('X');
  await expect.poll(() => editorText(page)).toContain('First marker repeat.X');
  await page.locator('.chapter-jump').filter({ hasText: 'Two' }).click();
  await expect.poll(async () => (await dimmedRanges(page)).join('\n')).toContain('First marker repeat.X');
  // ControlOrMeta, not Control: CodeMirror binds undo to Mod-z, which is Cmd on
  // macOS. A bare Control+z reaches the editor and does nothing there, so this
  // step passes on CI and fails for anyone running the suite on a Mac.
  await page.keyboard.press('ControlOrMeta+z');
  await expect.poll(() => editorText(page)).toBe(original);

  // Chapter scope is a filter on matches, so the same query answers
  // differently in the chapter and in the whole source.
  await page.locator('.chapter-jump').filter({ hasText: 'One' }).click();
  const findReplace = page.getByRole('group', { name: 'Find and replace' });
  await findReplace.getByLabel('Find').fill('Second marker');
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('No matches.');

  // reka-ui's Select renders a button[role=combobox] trigger plus a portaled
  // listbox, so open it and click the option instead of the native
  // <select>-only selectOption() API.
  await page.getByLabel('Scope').click();
  await page.getByRole('option', { name: 'Whole source' }).click();
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 1.');
  await expect.poll(() => editorSelectionText(page)).toBe('Second marker');

  // Following the caret into another chapter re-scopes the outline without
  // rewriting anything.
  await expect(page.locator('.chapter-list li.selected')).toHaveText(/Two/);

  await page.getByLabel('Scope').click();
  await page.getByRole('option', { name: 'Current chapter' }).click();
  await findReplace.getByLabel('Find').fill('repeat');
  await findReplace.getByLabel('Replace').fill('done');
  await findReplace.getByRole('button', { name: 'Replace all' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Replaced 1 occurrence.');
  await expect.poll(() => editorText(page)).toBe(
    original.replace('Second marker repeat.', 'Second marker done.')
  );

  await page.getByRole('button', { name: 'Whole source' }).click();
  await expect.poll(() => dimmedRanges(page)).toEqual([]);
  await page.getByRole('button', { name: 'Save*' }).click();
  await expect(page.getByText('Source saved.')).toBeVisible();

  await page.getByRole('button', { name: 'Back' }).click();
  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Manage sources' }).click();
  await expect(page.getByText('No pending changes').first()).toBeVisible();
  await expect(sourceEditor(page)).toBeVisible();
  await expect.poll(() => editorText(page)).toBe(
    original.replace('Second marker repeat.', 'Second marker done.')
  );
});

test('should not move the view while typing into a source taller than the screen', async ({ page }) => {
  const { baseUrl } = getServer();

  await openSourceEditor(page, baseUrl, 'srceditor-typing');

  const filler = Array.from(
    { length: 60 },
    (_, index) => `Filler line ${index + 1}: ${'wrapped content '.repeat(24)}`
  ).join('\n');
  await setEditorText(page, `${filler}\nTyping anchor\n${filler}`);

  // Park the caret deep inside the document, where the editor has no reason
  // of its own to scroll while typing.
  const findReplace = page.getByRole('group', { name: 'Find and replace' });
  await findReplace.getByLabel('Find').fill('Typing anchor');
  await findReplace.getByRole('button', { name: 'Next' }).click();
  await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 1.');
  await expect.poll(() => editorScrollTop(page)).toBeGreaterThan(1_000);

  // Collapse the match and lift the caret well clear of both edges, so that
  // nothing has to scroll for it to stay visible.
  await page.keyboard.press('ArrowRight');
  await scrollEditorTo(page, (await editorScrollTop(page)) + 200);
  const parked = await editorScrollTop(page);

  for (const character of 'typed') {
    await page.keyboard.type(character);
    await page.waitForTimeout(120);
    expect(await editorScrollTop(page)).toBe(parked);
  }
  await expect.poll(() => editorText(page)).toMatch(/Typing anchortyped/);
});
