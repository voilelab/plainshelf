import { expect, type Page } from '@playwright/test';

export const DESKTOP_PREVIEW_QUERY = 'desktop-shell-preview=1';

function withDesktopPreview(route: string): string {
  const separator = route.includes('?') ? '&' : '?';
  return `${route}${separator}${DESKTOP_PREVIEW_QUERY}`;
}

export async function openDesktopAt(page: Page, baseUrl: string, route: string): Promise<void> {
  await page.goto(`${baseUrl}${withDesktopPreview(route)}`);
}

export function desktopHistoryControls(page: Page) {
  return page.getByRole('navigation', { name: 'Desktop history navigation' });
}

export async function expectDesktopShellEngaged(page: Page): Promise<void> {
  await expect(desktopHistoryControls(page)).toBeVisible();
}
