import { expect, type Page } from '@playwright/test';

// Mirrors frontend/src/providers/runtime.ts isWailsRuntime()'s browser escape
// hatch, the desktop counterpart of MOBILE_PREVIEW_QUERY in ./mobile.ts:
// appending this to a top-level goto makes the app boot in desktop-shell mode
// on ordinary Chromium, so the Wails-only branches (history controls, shelf
// management in Settings) get exercised without building the Wails binary.
export const DESKTOP_PREVIEW_QUERY = 'desktop-shell-preview=1';

function withDesktopPreview(route: string): string {
  const separator = route.includes('?') ? '&' : '?';
  return `${route}${separator}${DESKTOP_PREVIEW_QUERY}`;
}

/** Opens `route` with the desktop-shell preview flag set. */
export async function openDesktopAt(page: Page, baseUrl: string, route: string): Promise<void> {
  await page.goto(`${baseUrl}${withDesktopPreview(route)}`);
}

/**
 * The desktop history pills rendered by App.vue. Present only while the app
 * considers itself to be running in the Wails shell, which makes this the
 * cheapest probe for "is desktop mode still engaged".
 */
export function desktopHistoryControls(page: Page) {
  return page.getByRole('navigation', { name: 'Desktop history navigation' });
}

export async function expectDesktopShellEngaged(page: Page): Promise<void> {
  await expect(desktopHistoryControls(page)).toBeVisible();
}
