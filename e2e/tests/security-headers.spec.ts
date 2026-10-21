import { expect, test } from '@playwright/test';
import { useServer } from './support/server';

const getServer = useServer();

test('the app boots with no CSP violations', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl } = getServer();

  const violations: string[] = [];
  page.on('console', (message) => {
    const text = message.text();
    if (text.includes('Content Security Policy') || text.includes('Content-Security-Policy')) {
      violations.push(text);
    }
  });

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  expect(violations, violations.join('\n')).toEqual([]);
});
