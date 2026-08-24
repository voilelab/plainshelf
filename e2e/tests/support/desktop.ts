import { expect, type Page } from '@playwright/test';

// Mirrors frontend/src/providers/runtime.ts isWailsRuntime()'s browser escape
// hatch, the desktop counterpart of MOBILE_PREVIEW_QUERY in ./mobile.ts:
// appending this to a top-level goto makes the app boot in desktop-shell mode
// on ordinary Chromium, so the Wails-only branches (the app-zoom hook, shelf
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
 * Whether the desktop shell is installed on the live page. main.ts calls
 * initAppZoom() at bootstrap, which registers window.__plainshelfZoom only when
 * isWailsRuntime() is true; web mode returns early and never sets it. That makes
 * the hook a route-independent probe for "the app booted as the desktop shell" —
 * the cheapest surviving stand-in for the retired history pills. Read fresh each
 * call so the assertion reflects the live window, not a cached value.
 */
export function desktopShellInstalled(page: Page): Promise<boolean> {
  return page.evaluate(
    () => typeof (window as { __plainshelfZoom?: unknown }).__plainshelfZoom === 'function'
  );
}

export async function expectDesktopShellEngaged(page: Page): Promise<void> {
  await expect.poll(() => desktopShellInstalled(page)).toBe(true);
}
