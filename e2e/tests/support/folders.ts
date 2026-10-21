import { expect, type Locator, type Page } from '@playwright/test';

export function foldersNav(page: Page): Locator {
  return page.getByRole('navigation', { name: 'Folders', exact: true });
}

export function foldersSectionToggle(page: Page): Locator {
  return page.getByRole('button', { name: 'Toggle sidebar folders', exact: true });
}

export async function expandFoldersSection(page: Page): Promise<void> {
  const toggle = foldersSectionToggle(page);
  if ((await toggle.getAttribute('aria-expanded')) === 'false') {
    await toggle.click();
  }
  await expect(foldersNav(page)).toBeVisible();
}

export function folderRow(page: Page, name: string): Locator {
  return foldersNav(page)
    .locator('.folder-node')
    .filter({ has: page.getByRole('button', { name, exact: true }) });
}

export async function openCreateFolderDialog(page: Page, parentName: string): Promise<Locator> {
  await expandFoldersSection(page);
  await openFolderContextMenu(page, parentName);
  await page.getByRole('menuitem', { name: 'Add folder', exact: true }).click();

  const dialog = page.getByRole('dialog', { name: 'New folder' });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function createFolder(page: Page, name: string, parentName: string): Promise<void> {
  const dialog = await openCreateFolderDialog(page, parentName);

  await dialog.getByLabel('Folder name').fill(name);
  await dialog.getByRole('button', { name: 'Create', exact: true }).click();

  await expect(dialog).not.toBeVisible();
}

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

export async function selectFolder(page: Page, name: string): Promise<void> {
  await folderRow(page, name).getByRole('button', { name, exact: true }).click();
}

export async function openFolderContextMenu(page: Page, name: string): Promise<void> {
  await folderRow(page, name).click({ button: 'right' });
}

export async function selectAllBooks(page: Page): Promise<void> {
  await foldersNav(page).getByRole('button', { name: 'All books', exact: true }).click();
}

export async function switchToCardView(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'List', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Card', exact: true }).click();
}

export async function emptyDataTransfer(page: Page) {
  return page.evaluateHandle(() => new DataTransfer());
}

export function foldersQueryRegex(path: string): RegExp {
  const escaped = path.split('/').join('(?:/|%2F)');
  return new RegExp(`[?&]folders=${escaped}(?:&|$)`);
}
