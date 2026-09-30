import { expect, type Locator, type Page } from '@playwright/test';

/** The sidebar folder tree (FolderTree.vue renders <nav aria-label="Folders">). */
export function foldersNav(page: Page): Locator {
  return page.getByRole('navigation', { name: 'Folders', exact: true });
}

/** The foldable Folders section header in the left sidebar. */
export function foldersSectionToggle(page: Page): Locator {
  return page.getByRole('button', { name: 'Toggle sidebar folders', exact: true });
}

/** Ensures the left sidebar's Folders section is expanded before using its controls. */
export async function expandFoldersSection(page: Page): Promise<void> {
  const toggle = foldersSectionToggle(page);
  if ((await toggle.getAttribute('aria-expanded')) === 'false') {
    await toggle.click();
  }
  await expect(foldersNav(page)).toBeVisible();
}

/**
 * Locates a single FolderNodeItem row by its exact visible name. A row only
 * contains its own buttons (descendant folders live in a sibling
 * `.tree-children` container), so the name filter cannot match children.
 */
export function folderRow(page: Page, name: string): Locator {
  return foldersNav(page)
    .locator('.folder-node')
    .filter({ has: page.getByRole('button', { name, exact: true }) });
}

/**
 * Opens folder creation from the requested parent row's context menu. The
 * shelf-root row is named "/"; nested callers pass the visible final segment.
 */
export async function openCreateFolderDialog(page: Page, parentName: string): Promise<Locator> {
  await expandFoldersSection(page);
  await openFolderContextMenu(page, parentName);
  await page.getByRole('menuitem', { name: 'Add folder', exact: true }).click();

  const dialog = page.getByRole('dialog', { name: 'New folder' });
  await expect(dialog).toBeVisible();
  return dialog;
}

/** Creates a single child folder from its parent's context menu. */
async function createFolder(page: Page, name: string, parentName: string): Promise<void> {
  const dialog = await openCreateFolderDialog(page, parentName);

  await dialog.getByLabel('Folder name').fill(name);
  await dialog.getByRole('button', { name: 'Create', exact: true }).click();

  await expect(dialog).not.toBeVisible();
}

/**
 * Creates the given path one level at a time from the shelf root and then each
 * newly created parent. Each successful submission navigates into the folder
 * it created, which is what this waits on.
 */
export async function addFolder(page: Page, path: string): Promise<void> {
  const segments = path.split('/').filter((segment) => segment.length > 0);
  let parent = '';

  for (const name of segments) {
    const parentName = parent === '' ? '/' : parent.split('/').at(-1) ?? '/';
    await createFolder(page, name, parentName);
    parent = parent === '' ? name : `${parent}/${name}`;
    await expect(page).toHaveURL(foldersQueryRegex(parent));
  }
}

/** Clicks a folder node's label button in the sidebar to filter by that folder. */
export async function selectFolder(page: Page, name: string): Promise<void> {
  await folderRow(page, name).getByRole('button', { name, exact: true }).click();
}

/** Right-clicks a folder node's row to open its context menu (rename/delete). */
export async function openFolderContextMenu(page: Page, name: string): Promise<void> {
  await folderRow(page, name).click({ button: 'right' });
}

/**
 * Clicks the fixed "All books" row in the folder tree sidebar to clear the
 * active folder filter. Scoped to the "Folders" nav because the library page
 * also renders an "All books" breadcrumb link while inside a nested folder.
 */
export async function selectAllBooks(page: Page): Promise<void> {
  await foldersNav(page).getByRole('button', { name: 'All books', exact: true }).click();
}

/**
 * Switches the book collection view mode from List (the default) to Card.
 * Only cards are draggable; list rows are not.
 */
export async function switchToCardView(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'List', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Card', exact: true }).click();
}

/**
 * Creates an empty DataTransfer handle shared across
 * dragstart/dragenter/dragover/drop, following the repo precedent in
 * import-book.spec.ts / books.ts. The app's own dragstart handlers call
 * setData on it (books: application/x-plainshelf-book-id, folders:
 * application/x-plainshelf-folder-path), so no manual setData is needed.
 */
export async function emptyDataTransfer(page: Page) {
  return page.evaluateHandle(() => new DataTransfer());
}

/**
 * Matches a `folders=` query parameter for the given slash-separated folder
 * path, tolerating either a literal "/" or a percent-encoded "%2F".
 */
export function foldersQueryRegex(path: string): RegExp {
  const escaped = path.split('/').join('(?:/|%2F)');
  return new RegExp(`[?&]folders=${escaped}(?:&|$)`);
}
