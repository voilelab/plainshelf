import { expect, test, type Locator, type Page } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { useServer } from './support/server';
import { openReaderTab } from './support/reader';
import {
  epubFixtureChapters,
  epubFixtureDescription,
  epubFixtureTitle,
  writeEpubFixture
} from './support/epub';

const getServer = useServer();

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

async function openReader(page: Page): Promise<Page> {
  await page.locator('.book-list-row').getByRole('heading', { name: epubFixtureTitle, exact: true }).click();
  // Playwright 1.62.1 popups inherit the context viewport, not the opener's page override.
  return openReaderTab(page, () => page.getByRole('button', { name: 'Start reading' }).click());
}

// Playwright 1.62.1 dispatchEvent uses createTouch/createTouchList with page coordinates on WebKit; new Touch throws Illegal constructor there (a2bf1cff).
interface LegacyTouchDocument extends Document {
  createTouch(
    view: Window,
    target: EventTarget,
    identifier: number,
    pageX: number,
    pageY: number,
    screenX: number,
    screenY: number
  ): Touch;
  createTouchList(...touches: Touch[]): TouchList;
}

async function dispatchTouchGesture(
  target: Locator,
  points: Array<{ x: number; y: number }>,
  pointerId = 1
): Promise<void> {
  await target.evaluate((element, { gesturePoints, identifier }) => {
    const doc = element.ownerDocument as LegacyTouchDocument;
    const legacy =
      typeof doc.createTouch === 'function' && typeof doc.createTouchList === 'function';

    const createTouch = (point: { x: number; y: number }) => {
      if (!legacy) {
        return new Touch({
          identifier,
          target: element,
          clientX: point.x,
          clientY: point.y,
          screenX: point.x,
          screenY: point.y
        });
      }
      return doc.createTouch(
        doc.defaultView as Window,
        element,
        identifier,
        point.x + (doc.scrollingElement?.scrollLeft ?? 0),
        point.y + (doc.scrollingElement?.scrollTop ?? 0),
        point.x,
        point.y
      );
    };
    const asTouches = (touches: Touch[]) => (legacy ? doc.createTouchList(...touches) : touches);
    const dispatch = (type: string, touches: Touch[], changedTouches: Touch[]) => {
      element.dispatchEvent(new TouchEvent(type, {
        bubbles: true,
        cancelable: true,
        touches: asTouches(touches),
        targetTouches: asTouches(touches),
        changedTouches: asTouches(changedTouches)
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

test('drives the mobile reader with real touch gestures', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${baseUrl}/books`);
  await importEpub(page);
  const reader = await openReader(page);
  await reader.setViewportSize({ width: 390, height: 844 });

  const mobileReader = reader.locator('[data-reader-variant="mobile"]');
  await expect(mobileReader).toBeVisible();

  const box = await mobileReader.boundingBox();
  expect(box).not.toBeNull();
  const center = { x: box!.x + box!.width / 2, y: box!.y + box!.height / 2 };

  await dispatchTouchGesture(mobileReader, [center, center]);
  await expect(mobileReader.locator('.mobile-reader-toolbar')).toBeVisible();
  await expect(mobileReader.getByText('1 / 3')).toBeVisible();
  await expect(mobileReader.getByText(epubFixtureDescription)).toBeVisible();

  await dispatchTouchGesture(mobileReader, [
    { x: box!.x + box!.width * 0.8, y: center.y },
    { x: box!.x + box!.width * 0.5, y: center.y + 4 },
    { x: box!.x + box!.width * 0.2, y: center.y + 6 }
  ], 2);
  await expect(mobileReader.getByText('2 / 3')).toBeVisible();
  await expect(mobileReader.getByRole('heading', { name: epubFixtureChapters[0] })).toBeVisible();

  await dispatchTouchGesture(mobileReader, [
    { x: box!.x + box!.width * 0.2, y: center.y },
    { x: box!.x + box!.width * 0.5, y: center.y },
    { x: box!.x + box!.width * 0.8, y: center.y }
  ], 3);
  await expect(mobileReader.getByText('1 / 3')).toBeVisible();
  await expect(mobileReader.getByText(epubFixtureDescription)).toBeVisible();
});
