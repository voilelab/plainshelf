import { expect, type Locator, type Page } from '@playwright/test';


/** The sidebar layer tree (LayerTree.vue renders <nav aria-label="Layers">). */
export function layersNav(page: Page): Locator {
  return page.getByRole('navigation', { name: 'Layers', exact: true });
}

/** The foldable Layers section header in the left sidebar. */
export function layersSectionToggle(page: Page): Locator {
  return page.getByRole('button', { name: 'Toggle sidebar folders', exact: true });
}

/** Ensures the left sidebar's Layers section is expanded before using its controls. */
export async function expandLayersSection(page: Page): Promise<void> {
  const toggle = layersSectionToggle(page);
  if ((await toggle.getAttribute('aria-expanded')) === 'false') {
    await toggle.click();
  }
  await expect(layersNav(page)).toBeVisible();
}

/**
 * Locates a single LayerNodeItem row by its exact visible name. A row only
 * contains its own buttons (descendant layers live in a sibling
 * `.tree-children` container), so the name filter cannot match children.
 */
export function layerRow(page: Page, name: string): Locator {
  return layersNav(page)
    .locator('.layer-node')
    .filter({ has: page.getByRole('button', { name, exact: true }) });
}

/**
 * Opens the "Add layer" inline form, submits the given path in one go
 * (which can be a nested path like "novels/scifi"), and waits for the
 * "Layer created" success confirmation.
 */
export async function addLayer(page: Page, path: string): Promise<void> {
  await expandLayersSection(page);
  await page.getByRole('button', { name: 'Add layer', exact: true }).click();
  await page.getByPlaceholder('e.g. programming/rust').fill(path);
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  await expect(page.getByText('Layer created')).toBeVisible();
}

/** Clicks a layer node's label button in the sidebar to filter by that layer. */
export async function selectLayer(page: Page, name: string): Promise<void> {
  await layerRow(page, name).getByRole('button', { name, exact: true }).click();
}

/** Right-clicks a layer node's row to open its context menu (rename/delete). */
export async function openLayerContextMenu(page: Page, name: string): Promise<void> {
  await layerRow(page, name).click({ button: 'right' });
}

/**
 * Clicks the fixed "All books" row in the layer tree sidebar to clear the
 * active layer filter. Scoped to the "Layers" nav because the library page
 * also renders an "All books" breadcrumb link while inside a nested layer.
 */
export async function selectAllBooks(page: Page): Promise<void> {
  await layersNav(page).getByRole('button', { name: 'All books', exact: true }).click();
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
 * setData on it (books: application/x-plainshelf-book-id, layers:
 * application/x-plainshelf-layer-path), so no manual setData is needed.
 */
export async function emptyDataTransfer(page: Page) {
  return page.evaluateHandle(() => new DataTransfer());
}

/**
 * Matches a `layers=` query parameter for the given slash-separated layer
 * path, tolerating either a literal "/" or a percent-encoded "%2F".
 */
export function layersQueryRegex(path: string): RegExp {
  const escaped = path.split('/').join('(?:/|%2F)');
  return new RegExp(`[?&]layers=${escaped}(?:&|$)`);
}
