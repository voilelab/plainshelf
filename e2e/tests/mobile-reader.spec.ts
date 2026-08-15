import { expect, test, type Locator, type Page } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { startServer } from './support/server';
import {
  epubFixtureChapters,
  epubFixtureDescription,
  epubFixtureTitle,
  writeEpubFixture
} from './support/epub';

let fixtureDir = '';
let epubFixturePath = '';

test.beforeAll(() => {
  fixtureDir = fs.mkdtempSync(path.join(os.tmpdir(), 'plainshelf-mobile-reader-'));
  epubFixturePath = writeEpubFixture(fixtureDir);
});

test.afterAll(() => {
  if (fixtureDir) fs.rmSync(fixtureDir, { recursive: true, force: true });
});

async function importEpub(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Import/ }).click();
  await page.getByRole('menuitem', { name: 'Import from files' }).click();
  const dialog = page.getByRole('dialog', { name: 'Import Book' });
  await dialog.locator('input[type="file"]').setInputFiles(epubFixturePath);
  const importButton = dialog.getByRole('button', { name: 'Import', exact: true });
  await importButton.click();
  await expect(importButton).toBeDisabled();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
}

async function openReader(page: Page): Promise<void> {
  await page.locator('.book-list-row').getByRole('heading', { name: epubFixtureTitle, exact: true }).click();
  await page.getByRole('button', { name: 'Start reading' }).click();
  await expect(page).toHaveURL(/\/reader\/[^/]+$/);
}

async function dispatchTouch(
  target: Locator,
  points: Array<{ x: number; y: number }>,
  pointerId = 1
): Promise<void> {
  await target.evaluate((element, { gesturePoints, identifier }) => {
    const createTouch = (point: { x: number; y: number }) => new Touch({
      identifier,
      target: element,
      clientX: point.x,
      clientY: point.y,
      screenX: point.x,
      screenY: point.y
    });
    const dispatch = (type: string, touches: Touch[], changedTouches: Touch[]) => {
      element.dispatchEvent(new TouchEvent(type, {
        bubbles: true,
        cancelable: true,
        touches,
        targetTouches: touches,
        changedTouches
      }));
    };

    const firstTouch = createTouch(gesturePoints[0]);
    dispatch('touchstart', [firstTouch], [firstTouch]);
    for (const point of gesturePoints.slice(1, -1)) {
      const moveTouch = createTouch(point);
      dispatch('touchmove', [moveTouch], [moveTouch]);
    }
    const lastTouch = createTouch(gesturePoints[gesturePoints.length - 1]);
    dispatch('touchend', [], [lastTouch]);
  }, { gesturePoints: points, identifier: pointerId });
}

test('provides an immersive mobile reader without changing the desktop reader', async ({ page }) => {
  const server = await startServer();

  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${server.baseUrl}/books`);
    await importEpub(page);
    await openReader(page);

    const mobileReader = page.locator('[data-reader-variant="mobile"]');
    const readerText = mobileReader.locator('.reader-text');
    await expect(mobileReader).toBeVisible();
    await expect(readerText).toHaveCSS('font-size', '22px');
    await expect(page.getByText('Tap the center for controls · Swipe left or right to change chapters')).toBeVisible();
    await expect.poll(() => page.evaluate(() => localStorage.getItem('reader-mobile-gesture-hint-seen'))).toBe('1');
    await expect(mobileReader.getByRole('button', { name: 'Next' })).toHaveCount(0);
    await expect(mobileReader.locator('.mobile-reader-toolbar')).toHaveCount(0);

    const box = await mobileReader.boundingBox();
    expect(box).not.toBeNull();
    const center = { x: box!.x + box!.width / 2, y: box!.y + box!.height / 2 };
    await dispatchTouch(mobileReader, [center, center]);
    await expect(mobileReader.locator('.mobile-reader-toolbar')).toBeVisible();
    await expect(mobileReader.getByText('1 / 3')).toBeVisible();
    await page.waitForTimeout(4_100);
    await expect(mobileReader.locator('.mobile-reader-toolbar')).toBeVisible();

    await page.keyboard.press('ArrowRight');
    await expect(mobileReader.getByText('2 / 3')).toBeVisible();
    await page.keyboard.press('ArrowLeft');
    await expect(mobileReader.getByText('1 / 3')).toBeVisible();

    const keyboardInput = mobileReader.locator('input[data-keyboard-guard]');
    await mobileReader.evaluate((element) => {
      const input = document.createElement('input');
      input.dataset.keyboardGuard = 'true';
      element.append(input);
      input.focus();
    });
    await page.keyboard.press('ArrowRight');
    await expect(mobileReader.getByText('1 / 3')).toBeVisible();
    await keyboardInput.evaluate((element) => element.remove());

    await mobileReader.getByRole('button', { name: 'Choose reading font' }).click();
    const fontDialog = page.getByRole('dialog', { name: 'Reading font' });
    await expect(fontDialog).toBeVisible();
    await page.keyboard.press('ArrowRight');
    await expect(mobileReader.getByText('1 / 3')).toBeVisible();
    await fontDialog.getByRole('button', { name: 'Done' }).click();
    await expect(mobileReader.locator('.mobile-reader-toolbar')).toBeVisible();
    await dispatchTouch(mobileReader, [center, center], 6);
    await expect(mobileReader.locator('.mobile-reader-toolbar')).toHaveCount(0);
    await dispatchTouch(mobileReader, [center, center], 7);
    await expect(mobileReader.locator('.mobile-reader-toolbar')).toBeVisible();

    await dispatchTouch(mobileReader, [
      { x: box!.x + box!.width * 0.8, y: center.y },
      { x: box!.x + box!.width * 0.5, y: center.y + 4 },
      { x: box!.x + box!.width * 0.2, y: center.y + 6 }
    ], 2);
    await expect(mobileReader.getByText('2 / 3')).toBeVisible();
    await expect(mobileReader.getByRole('heading', { name: epubFixtureChapters[0] })).toBeVisible();

    const codeBlock = mobileReader.locator('.reader-md-code');
    await expect(codeBlock).toBeVisible();
    await expect(codeBlock).toHaveCSS('touch-action', /^(?:manipulation|pan-x pan-y pinch-zoom)$/);
    await expect.poll(() => codeBlock.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true);
    const codeBox = await codeBlock.boundingBox();
    expect(codeBox).not.toBeNull();
    await dispatchTouch(codeBlock, [
      { x: codeBox!.x + codeBox!.width * 0.8, y: codeBox!.y + codeBox!.height / 2 },
      { x: codeBox!.x + codeBox!.width * 0.2, y: codeBox!.y + codeBox!.height / 2 }
    ], 11);
    await expect(mobileReader.getByText('2 / 3')).toBeVisible();
    await codeBlock.evaluate((element) => {
      element.scrollLeft = element.scrollWidth;
    });
    await expect.poll(() => codeBlock.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);

    await dispatchTouch(mobileReader, [
      { x: center.x, y: box!.y + box!.height * 0.75 },
      { x: center.x + 4, y: box!.y + box!.height * 0.45 },
      { x: center.x + 6, y: box!.y + box!.height * 0.25 }
    ], 3);
    await expect(mobileReader.getByText('2 / 3')).toBeVisible();

    await dispatchTouch(mobileReader, [
      { x: box!.x + box!.width * 0.2, y: center.y },
      { x: box!.x + box!.width * 0.5, y: center.y },
      { x: box!.x + box!.width * 0.8, y: center.y }
    ], 4);
    await expect(mobileReader.getByText('1 / 3')).toBeVisible();
    await expect(mobileReader.getByText(epubFixtureDescription)).toBeVisible();

    await dispatchTouch(mobileReader, [
      { x: box!.x + box!.width * 0.2, y: center.y },
      { x: box!.x + box!.width * 0.8, y: center.y }
    ], 5);
    await expect(page.getByText('You are at the first chapter').last()).toBeVisible();

    for (const [index, pointerId] of [8, 9].entries()) {
      await dispatchTouch(mobileReader, [
        { x: box!.x + box!.width * 0.8, y: center.y },
        { x: box!.x + box!.width * 0.2, y: center.y }
      ], pointerId);
      await expect(mobileReader.getByText(`${index + 2} / 3`)).toBeVisible();
    }
    await dispatchTouch(mobileReader, [
      { x: box!.x + box!.width * 0.8, y: center.y },
      { x: box!.x + box!.width * 0.2, y: center.y }
    ], 10);
    await expect(page.getByText('You are at the last chapter').last()).toBeVisible();

    await page.reload();
    await expect(page.locator('[data-reader-variant="mobile"]')).toBeVisible();
    await expect(page.getByText('Tap the center for controls · Swipe left or right to change chapters')).toHaveCount(0);

    await page.setViewportSize({ width: 1280, height: 720 });
    const desktopReader = page.locator('[data-reader-variant="desktop"]');
    await expect(desktopReader).toBeVisible();
    await expect(desktopReader.getByRole('button', { name: 'Prev' })).toBeVisible();
    await expect(desktopReader.getByRole('button', { name: 'Next' })).toBeVisible();
    await expect(desktopReader.locator('.reader-text')).toHaveCSS('font-size', '20px');
  } finally {
    await server.dispose();
  }
});
