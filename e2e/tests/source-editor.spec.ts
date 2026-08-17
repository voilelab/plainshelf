import { expect, test, type Page } from '@playwright/test';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

async function findBookPackage(root: string, bookId: string): Promise<string> {
  for (const entry of await fs.readdir(root, { withFileTypes: true })) {
    const entryPath = path.join(root, entry.name);
    if (!entry.isDirectory()) continue;
    if (entry.name.endsWith('.bookpkg')) {
      const meta = JSON.parse(await fs.readFile(path.join(entryPath, 'book.json'), 'utf8')) as { id?: string };
      if (meta.id === bookId) return entryPath;
    }
    const found = await findBookPackage(entryPath, bookId).catch(() => '');
    if (found) return found;
  }
  throw new Error(`Book package ${bookId} not found below ${root}`);
}

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
      textarea: rect('.source-content-textarea')
    };
  });

  expect(metrics.page.bottom).toBeLessThanOrEqual(metrics.viewportHeight + 1);
  expect(metrics.editor.bottom).toBeLessThanOrEqual(metrics.page.bottom + 1);
  expect(metrics.textarea.top).toBeGreaterThanOrEqual(metrics.editor.top - 1);
  expect(metrics.textarea.bottom).toBeLessThanOrEqual(metrics.editor.bottom + 1);
}

test('should edit source content and see the change reflected in the reader', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Open detail page then navigate to source editor
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+\/sources$/);

    // Wait for the source to finish loading
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    const textarea = page.locator('.source-content-textarea');
    await expect(textarea).toBeEnabled();
    await expectSourceEditorFitsViewport(page);

    await page.setViewportSize({ width: 800, height: 500 });
    await expectSourceEditorFitsViewport(page);
    await page.setViewportSize({ width: 1280, height: 720 });
    await expectSourceEditorFitsViewport(page);

    // Append unique content to the existing source
    const original = await textarea.inputValue();
    const filler = Array.from(
      { length: 40 },
      (_, index) => `Filler line ${index + 1}: ${'wrapped content '.repeat(24)}`
    ).join('\n');
    await textarea.fill(`${original}\nrepeat repeat\n${filler}\nDeep search marker.\nEdited by E2E source editor.`);

    const findReplace = page.getByRole('group', { name: 'Find and replace' });
    await findReplace.getByLabel('Find').fill('PlainShelf');
    await findReplace.getByRole('button', { name: 'Next' }).click();
    await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 1.');
    await expect.poll(() => textarea.evaluate((element) => {
      const field = element as HTMLTextAreaElement;
      return field.value.slice(field.selectionStart, field.selectionEnd);
    })).toBe('PlainShelf');

    await findReplace.getByLabel('Find').fill('Deep search marker');
    await findReplace.getByRole('button', { name: 'Next' }).click();
    // Newline-count scrolling lands around 1,100px here, but these long lines
    // wrap into several visual rows. The mirror measurement must include those
    // rows and therefore move substantially farther down.
    await expect.poll(() => textarea.evaluate((element) => (element as HTMLTextAreaElement).scrollTop)).toBeGreaterThan(4_000);
    await expect.poll(() => textarea.evaluate((element) => {
      const field = element as HTMLTextAreaElement;
      return field.value.slice(field.selectionStart, field.selectionEnd);
    })).toBe('Deep search marker');
    await textarea.evaluate((element) => {
      const field = element as HTMLTextAreaElement;
      field.setSelectionRange(0, 0);
      field.scrollTop = 0;
    });

    // Replace must find and replace in one click even when another match is
    // currently selected; it must not require a separate Find click first.
    await findReplace.getByLabel('Find').fill('E2E');
    await findReplace.getByLabel('Replace').fill('browser');
    await findReplace.getByRole('button', { name: 'Replace', exact: true }).click();
    await expect(textarea).toHaveValue(/Hello from PlainShelf browser\./);
    await expect(findReplace.getByRole('status')).toHaveText('Replaced 1 occurrence. Match 1 of 1.');

    await findReplace.getByLabel('Find').fill('repeat');
    await findReplace.getByLabel('Replace').fill('done');
    await findReplace.getByRole('button', { name: 'Replace all' }).click();
    await expect(textarea).toHaveValue(/done done/);
    await expect(findReplace.getByRole('status')).toHaveText('Replaced 2 occurrences.');

    // Status should flip to "Unsaved changes" and Save button should enable
    await expect(page.getByText('Unsaved changes').first()).toBeVisible();
    const saveButton = page.getByRole('button', { name: 'Save*' });
    await expect(saveButton).toBeEnabled();
    await saveButton.click();

    // After save the topbar shows "Source saved."
    await expect(page.getByText('Source saved.')).toBeVisible();

    // Go back to the detail page, then open the reader
    await page.getByRole('button', { name: 'Back' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: /reading/i }).click();
    await expect(page).toHaveURL(/\/reader\/[^/]+$/);

    // Reader should display the appended line
    await expect(page.getByText('Edited by E2E source editor.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should create a new source, set it as current, and see its content in the reader', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Open source editor
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+\/sources$/);

    // Wait for initial source to load
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    // Create a new source
    const newBtn = page.getByRole('button', { name: 'New' });
    await newBtn.click();

    // Wait for the entire creation cycle to settle:
    //   - "New" button re-enabled means creating=false and loadSource finished
    //   - textarea enabled and empty confirms the new source is active
    //   - "No pending changes" confirms isDirty=false (content===initialContent==='')
    await expect(newBtn).toBeEnabled();
    const textarea = page.locator('.source-content-textarea');
    await expect(textarea).toBeEnabled();
    await expect(textarea).toHaveValue('');
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    // Type unique content; use keyboard.type so Vue's controlled :value binding
    // stays in sync across individual input events rather than a single bulk fill
    await textarea.click();
    await page.keyboard.type('This is the second source.');

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

    // Open the reader and verify the new source is rendered
    await page.getByRole('button', { name: 'Back' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'Start reading' }).click();
    await expect(page).toHaveURL(/\/reader\/[^/]+$/);

    await expect(page.getByText('This is the second source.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should derive chapterized Markdown from TXT and keep the original source', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page.getByText('No pending changes').first()).toBeVisible();

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
    const textarea = page.locator('.source-content-textarea');
    await expect(textarea).toHaveValue(/## Untitled chapter/);
    await page.getByRole('button', { name: 'Save*' }).click();
    await expect(page.getByText('Source saved.')).toBeVisible();

    await page.getByRole('button', { name: 'Back' }).click();
    await page.getByRole('button', { name: 'Start reading' }).click();
    await expect(page.getByText('1 / 2')).toBeVisible();
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByRole('heading', { name: 'Untitled chapter' })).toBeVisible();
    await expect(page.getByText('This text came from a real uploaded TXT file.')).toBeVisible();

    await page.getByRole('link', { name: 'Back to detail' }).click();
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    const originalSource = page.locator('.source-item').filter({ hasNotText: 'Current' });
    await originalSource.locator('.source-item-content').click();
    await page.getByRole('button', { name: 'Set as current' }).click();
    await expect(page.getByText('Current source updated.')).toBeVisible();

    await page.getByRole('button', { name: 'Back' }).click();
    await page.getByRole('button', { name: /reading/i }).click();
    await expect(page.getByText('1 / 1')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Untitled chapter' })).toHaveCount(0);
    await expect(page.getByText('This text came from a real uploaded TXT file.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should focus one Markdown chapter while preserving and searching the whole source', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    await page.getByRole('button', { name: 'Manual TXT → MD' }).click();
    const conversionDialog = page.getByRole('dialog', { name: 'Create chapterized Markdown source' });
    await conversionDialog.getByRole('button', { name: 'Create source' }).click();
    await expect(page.getByText('Derived source created.')).toBeVisible();

    const textarea = page.locator('.source-content-textarea');
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
    await textarea.fill(original);

    await page.setViewportSize({ width: 700, height: 700 });
    await page.getByRole('button', { name: 'Chapters', exact: true }).click();
    await page.locator('.chapter-jump').filter({ hasText: 'One' }).click();
    await expect(textarea).toBeVisible();
    await expect(textarea).toHaveValue(/^## One\nFirst marker repeat\.\n\n$/);
    await expect(textarea).not.toHaveValue(/Opening marker|Second marker/);
    await expect(page.getByLabel('Scope')).toHaveValue('section');
    await page.setViewportSize({ width: 1280, height: 720 });

    const composition = await textarea.evaluate(async (element) => {
      const field = element as HTMLTextAreaElement;
      field.focus();
      field.dispatchEvent(new CompositionEvent('compositionstart', {
        bubbles: true,
        data: ''
      }));
      field.value = `${field.value}中文`;
      const composedEnd = field.value.length;
      field.setSelectionRange(composedEnd, composedEnd);
      field.dispatchEvent(new InputEvent('input', {
        bubbles: true,
        data: '中文',
        inputType: 'insertCompositionText',
        isComposing: true
      }));

      // Moving the range after the composing input exposes any next-tick
      // selection restore that would prematurely interfere with the IME.
      field.setSelectionRange(0, 0);
      await new Promise((resolve) => setTimeout(resolve, 0));
      const selectionDuringComposition = field.selectionStart;

      field.setSelectionRange(composedEnd, composedEnd);
      field.dispatchEvent(new CompositionEvent('compositionend', {
        bubbles: true,
        data: '中文'
      }));
      await new Promise((resolve) => setTimeout(resolve, 0));
      return {
        selectionDuringComposition,
        selectionAfterComposition: field.selectionStart,
        composedEnd,
        value: field.value
      };
    });
    expect(composition.selectionDuringComposition).toBe(0);
    expect(composition.selectionAfterComposition).toBe(composition.composedEnd);
    expect(composition.value).toContain('中文');

    // Pasting an H2 splits the visible chapter and follows the cursor into the
    // newly-created section without exposing the rest of the source.
    await textarea.fill([
      '## One',
      'First marker repeat.',
      'Chapter one edited.',
      '',
      '## Inserted',
      'Inserted marker.',
      ''
    ].join('\n'));
    await expect(textarea).toHaveValue(/^## Inserted\nInserted marker\.\n$/);
    await expect(textarea).not.toHaveValue(/## One|## Two/);

    const findReplace = page.getByRole('group', { name: 'Find and replace' });
    await findReplace.getByLabel('Find').fill('Second marker');
    await findReplace.getByRole('button', { name: 'Next' }).click();
    await expect(findReplace.getByRole('status')).toHaveText('No matches.');

    await page.getByLabel('Scope').selectOption('source');
    await findReplace.getByRole('button', { name: 'Next' }).click();
    await expect(textarea).toHaveValue(/^## Two\nSecond marker repeat\.$/);
    await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 1.');

    await page.getByRole('button', { name: 'Whole source' }).click();
    const draftBeforeReplace = await textarea.inputValue();
    expect(draftBeforeReplace).toBe([
      '# Focused book',
      'Opening marker.',
      '',
      '## One',
      'First marker repeat.',
      'Chapter one edited.',
      '',
      '## Inserted',
      'Inserted marker.',
      '## Two',
      'Second marker repeat.'
    ].join('\n'));
    await page.locator('.chapter-jump').filter({ hasText: 'Two' }).click();
    await page.getByLabel('Scope').selectOption('source');

    await findReplace.getByLabel('Find').fill('repeat');
    await findReplace.getByLabel('Replace').fill('done');
    await findReplace.getByRole('button', { name: 'Replace all' }).click();
    await expect(findReplace.getByRole('status')).toHaveText('Replaced 2 occurrences.');
    await expect(textarea).toHaveValue(/^## Two\nSecond marker done\.$/);

    await page.getByRole('button', { name: 'Whole source' }).click();
    await expect(textarea).toHaveValue(/Opening marker[\s\S]*First marker done[\s\S]*Chapter one edited[\s\S]*## Inserted[\s\S]*Second marker done/);
    await page.getByRole('button', { name: 'Save*' }).click();
    await expect(page.getByText('Source saved.')).toBeVisible();

    await page.getByRole('button', { name: 'Back' }).click();
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page.getByText('No pending changes').first()).toBeVisible();
    await expect(textarea).toHaveValue(/Opening marker[\s\S]*First marker done[\s\S]*## Inserted[\s\S]*Second marker done/);
  } finally {
    await server.dispose();
  }
});

test('should upgrade a legacy split source through the component modal', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    const bookId = new URL(page.url()).pathname.split('/').filter(Boolean).at(-1) ?? '';
    const bookPackage = await findBookPackage(server.shelfDir, bookId);
    const sourceId = (await fs.readdir(path.join(bookPackage, 'sources'), { withFileTypes: true }))
      .find((entry) => entry.isDirectory())?.name ?? '';
    const sourceMetaPath = path.join(bookPackage, 'sources', sourceId, 'meta.json');
    const sourceMeta = JSON.parse(await fs.readFile(sourceMetaPath, 'utf8')) as Record<string, unknown>;
    delete sourceMeta.schema_version;
    delete sourceMeta.format;
    sourceMeta.split_config = { type: 'regex', regex: '^Hello from PlainShelf E2E\\.$' };
    await fs.writeFile(sourceMetaPath, `${JSON.stringify(sourceMeta, null, 2)}\n`, 'utf8');

    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page.getByText('Legacy', { exact: true }).first()).toBeVisible();
    await page.getByRole('button', { name: 'Upgrade chapter format' }).click();

    const dialog = page.getByRole('dialog', { name: 'Upgrade chapter format' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('1 H2 chapter will be created')).toBeVisible();
    await expect(dialog.locator('pre')).toContainText('## Hello from PlainShelf E2E.');
    await dialog.getByRole('button', { name: 'Create source' }).click();

    await expect(page.getByText('Derived source created.')).toBeVisible();
    await expect(page.getByText('2 total')).toBeVisible();
    await expect(page.getByText('MD', { exact: true }).first()).toBeVisible();
    await expect(page.locator('.source-content-textarea')).toHaveValue(/^## Hello from PlainShelf E2E\./);

    await page.getByRole('button', { name: 'Back' }).click();
    await page.getByRole('button', { name: 'Start reading' }).click();
    await expect(page.getByRole('heading', { name: 'Hello from PlainShelf E2E.' })).toBeVisible();
    await expect(page.getByText('## Hello from PlainShelf E2E.')).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('should not move the view while typing into a source taller than the screen', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Manage sources' }).click();
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    const textarea = page.locator('.source-content-textarea');
    await expect(textarea).toBeEnabled();

    const filler = Array.from(
      { length: 60 },
      (_, index) => `Filler line ${index + 1}: ${'wrapped content '.repeat(24)}`
    ).join('\n');
    await textarea.fill(`${filler}\nTyping anchor\n${filler}`);

    // Park the caret deep inside the document, where the browser has no reason
    // of its own to scroll while typing.
    const findReplace = page.getByRole('group', { name: 'Find and replace' });
    await findReplace.getByLabel('Find').fill('Typing anchor');
    await findReplace.getByRole('button', { name: 'Next' }).click();
    await expect(findReplace.getByRole('status')).toHaveText('Match 1 of 1.');

    const scrollTop = () => textarea.evaluate((element) => (element as HTMLTextAreaElement).scrollTop);
    await expect.poll(scrollTop).toBeGreaterThan(1_000);

    // Collapse the match and lift the caret close to the top edge: still fully
    // visible, so nothing should scroll, but near enough to the edge that a
    // caret-revealing pass would jump the view.
    await textarea.press('ArrowRight');
    await textarea.evaluate((element) => {
      const field = element as HTMLTextAreaElement;
      field.scrollTop += field.clientHeight / 3 - 20;
    });
    const parked = await scrollTop();

    for (const character of 'typed') {
      await page.keyboard.type(character);
      await page.waitForTimeout(120);
      expect(await scrollTop()).toBe(parked);
    }
    await expect(textarea).toHaveValue(/Typing anchortyped/);
  } finally {
    await server.dispose();
  }
});
