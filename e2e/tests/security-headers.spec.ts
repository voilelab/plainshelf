import { expect, test } from '@playwright/test';
import { useServer } from './support/server';

const getServer = useServer();

// The browser-hardening headers the server sends on every response. Restated
// here rather than derived: this test's job is to pin the exact names and
// values a real HTTP response carries, so it fails if any of them drifts.
const staticSecurityHeaders: Record<string, string> = {
  'x-content-type-options': 'nosniff',
  'x-frame-options': 'DENY',
  'referrer-policy': 'no-referrer'
};

// The document (index.html) and an API response are the two response shapes the
// ticket names; checking both proves the headers ride on the whole surface, not
// just one handler.
test('every response carries the static security headers', async () => {
  const { baseUrl } = getServer();

  for (const path of ['/', '/api/version']) {
    const response = await fetch(`${baseUrl}${path}`);
    for (const [name, value] of Object.entries(staticSecurityHeaders)) {
      expect(response.headers.get(name), `${path} header ${name}`).toBe(value);
    }
  }
});

test('the index document carries a nonce-based CSP the app can boot under', async () => {
  const { baseUrl } = getServer();

  const response = await fetch(`${baseUrl}/`);
  const csp = response.headers.get('content-security-policy');
  expect(csp, 'index CSP header').not.toBeNull();
  const policy = csp ?? '';

  // The directives the ticket names as the required floor.
  for (const directive of [
    "default-src 'self'",
    "object-src 'none'",
    "frame-ancestors 'none'",
    "base-uri 'self'"
  ]) {
    expect(policy, 'CSP directives').toContain(directive);
  }

  // script-src admits the inline bootstrap by nonce, never by 'unsafe-inline'.
  const scriptSrc = /script-src ([^;]*)/.exec(policy)?.[1] ?? '';
  expect(scriptSrc, 'script-src').not.toContain("'unsafe-inline'");
  const headerNonce = /'nonce-([^']+)'/.exec(scriptSrc)?.[1];
  expect(headerNonce, 'script-src nonce').toBeTruthy();

  // The nonce the header announces is the nonce written on the injected
  // bootstrap script, so the one inline script the app needs is exactly the one
  // the policy admits.
  const body = await response.text();
  const bodyNonce = /<script nonce="([^"]+)">window\.__PLAINSHELF_SECURITY__/.exec(body)?.[1];
  expect(bodyNonce, 'bootstrap script nonce').toBe(headerNonce);
});

// A CSP is only as good as the app's ability to actually run under it: if a real
// browser boot logged a CSP violation, some resource the UI needs would be
// blocked. This drives a full page load and fails on any violation report.
test('the app boots with no CSP violations', async ({ page }) => {
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
