import { expect, type Page } from '@playwright/test';

/**
 * On a plain web-server build `goRead` opens the reader in a new browser tab
 * (window.open with noopener/noreferrer — see
 * frontend/src/composables/useBookActions.ts), instead of navigating the
 * current page in place. The e2e suite runs exactly that build, so every read
 * entry point now yields a second tab.
 *
 * This runs the given read affordance and returns the reader's own Page, already
 * on the /reader/:id route. `open` runs inside the same Promise.all as the
 * 'page' wait so the new tab is awaited before the click settles and can never
 * be missed. Pass `urlPattern` when the route carries a query (e.g. ?section=).
 *
 * A noopener popup is still a page in the same browser context, so it shares the
 * origin's localStorage with the opener — read history and reading progress
 * written by the reader are visible to the original tab.
 */
export async function openReaderTab(
  page: Page,
  open: () => Promise<void>,
  urlPattern: RegExp = /\/reader\/[^/]+$/
): Promise<Page> {
  const [reader] = await Promise.all([page.context().waitForEvent('page'), open()]);
  await reader.waitForLoadState('domcontentloaded');
  await expect(reader).toHaveURL(urlPattern);
  return reader;
}
