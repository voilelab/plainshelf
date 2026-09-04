import { expect, test } from '@playwright/test';
import { useServer } from './support/server';

const getServer = useServer();

// The header names, the CSP directives and the bootstrap nonce are pinned at L2
// by server/contract/crosscut/security_headers_test.go, which runs the same
// handler over httptest and covers more response shapes than a browser reaches.
// What only a browser can answer is whether the app actually runs under that
// policy: a real boot logging a CSP violation means some resource the UI needs
// is blocked. That is the one case left here.
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
