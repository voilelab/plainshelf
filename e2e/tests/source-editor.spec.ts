import { expect, test, type Page } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath } from './support/books';
import { openReaderTab } from './support/reader';
import {
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

test('should edit source content and see the change reflected in the reader', { tag: '@smoke' }, async ({ page }) => {
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

test('should create a new source, set it as current, and see its content in the reader', { tag: '@smoke' }, async ({
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
