import { expect, test } from '@playwright/test';
import { importBookFromPath, helloMarkdownFixturePath } from './support/books';
import { startServer } from './support/server';

test('should switch and persist the reading font without changing code text', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloMarkdownFixturePath);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await page.getByRole('button', { name: 'Start reading' }).click();
    await expect(page).toHaveURL(/\/reader\/[^/]+$/);

    const readerText = page.locator('.reader-text');
    const markdownHeading = page.locator('.reader-md-h1');
    const inlineCode = page.locator('.reader-md-inline-code');

    await expect(readerText).toHaveCSS('font-family', /Georgia/);
    await page.getByRole('button', { name: 'Choose reading font' }).click();

    const dialog = page.getByRole('dialog', { name: 'Reading font' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('radio', { name: /System serif/ })).toBeChecked();

    await dialog.getByRole('radio', { name: /Noto Sans TC/ }).check();
    await expect(readerText).toHaveCSS('font-family', /Noto Sans TC Variable/);
    await expect(markdownHeading).toHaveCSS('font-family', /Noto Sans TC Variable/);
    await expect(inlineCode).toHaveCSS('font-family', /monospace/);
    await expect.poll(() => page.evaluate(() => localStorage.getItem('reader-font-family'))).toBe('noto-sans-tc');

    await dialog.getByRole('radio', { name: /Noto Serif TC/ }).check();
    await expect(readerText).toHaveCSS('font-family', /Noto Serif TC Variable/);
    await expect.poll(() => page.evaluate(() => localStorage.getItem('reader-font-family'))).toBe('noto-serif-tc');
    await dialog.getByRole('button', { name: 'Done' }).click();

    await page.reload();
    await expect(readerText).toHaveCSS('font-family', /Noto Serif TC Variable/);
    await page.getByRole('button', { name: 'Choose reading font' }).click();
    const reopenedDialog = page.getByRole('dialog', { name: 'Reading font' });
    await expect(reopenedDialog.getByRole('radio', { name: /Noto Serif TC/ })).toBeChecked();
    await reopenedDialog.getByRole('button', { name: 'Done' }).click();

    await page.setViewportSize({ width: 320, height: 700 });
    const mobileReader = page.locator('[data-reader-variant="mobile"]');
    await expect(mobileReader).toBeVisible();
    const readerBox = await mobileReader.boundingBox();
    expect(readerBox).not.toBeNull();
    const center = { x: readerBox!.x + readerBox!.width / 2, y: readerBox!.y + readerBox!.height / 2 };
    await mobileReader.dispatchEvent('pointerdown', {
      pointerType: 'mouse', pointerId: 1, isPrimary: true, button: 0, clientX: center.x, clientY: center.y
    });
    await mobileReader.dispatchEvent('pointerup', {
      pointerType: 'mouse', pointerId: 1, isPrimary: true, button: 0, clientX: center.x, clientY: center.y
    });

    const toolbarButtons = page.locator('.mobile-reader-toolbar .mobile-reader-tool');
    await expect(toolbarButtons).toHaveCount(4);
    const buttonTops = await toolbarButtons.evaluateAll((buttons) => buttons.map((button) => button.getBoundingClientRect().top));
    expect(Math.max(...buttonTops) - Math.min(...buttonTops)).toBeLessThanOrEqual(1);

    await page.getByRole('button', { name: 'Choose reading font' }).click();
    const mobileDialogBox = await page.getByRole('dialog', { name: 'Reading font' }).boundingBox();
    expect(mobileDialogBox).not.toBeNull();
    expect(mobileDialogBox!.x).toBeGreaterThanOrEqual(0);
    expect(mobileDialogBox!.x + mobileDialogBox!.width).toBeLessThanOrEqual(320);

    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto(`${server.baseUrl}/settings`);
    await page.getByRole('tab', { name: 'About' }).click();
    await expect(page.getByText('Third-party font licenses')).toBeVisible();
    await expect(page.getByText('Noto Serif TC', { exact: true })).toBeVisible();
    await expect(page.getByText('Noto Sans TC', { exact: true })).toBeVisible();
    const licenseLinks = page.getByRole('link', { name: 'Full license text' });
    await expect(licenseLinks.nth(0)).toHaveAttribute('href', '/licenses/noto-serif-tc-OFL-1.1.txt');
    await expect(licenseLinks.nth(1)).toHaveAttribute('href', '/licenses/noto-sans-tc-OFL-1.1.txt');
  } finally {
    await server.dispose();
  }
});
