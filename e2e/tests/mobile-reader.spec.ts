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
  // The web build opens the reader in a new tab. A popup inherits the browser
  // context's default viewport, not this page's mobile override, so the caller
  // must size the returned reader tab itself.
  return openReaderTab(page, () => page.getByRole('button', { name: 'Start reading' }).click());
}

// WebKit is the second engine the nightly runs (PSW-111), and it does not
// expose a constructible `Touch`: `new Touch(...)` throws "Illegal
// constructor" there. Its equivalents are the legacy `document.createTouch`
// and `document.createTouchList`, which take *page* coordinates and return a
// real `TouchList` — the same split Playwright makes inside its own
// `dispatchEvent`. Chromium dropped both, so their presence is the switch.
//
// The whole gesture stays inside one `evaluate` on purpose: a tap is only a
// tap while touchstart and touchend are under 350ms apart
// (`MOBILE_READER_TAP_DURATION_MS`), which one protocol round-trip per event
// would put at the mercy of a loaded runner.
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

async function dispatchTouch(
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

/*
What is left here is the part no lower level reaches: a real TouchEvent, in a
real engine, driving a real EPUB that was imported through the API and written
to a shelf on disk. Everything this case used to assert about the reader's own
behaviour — the hint and its timer, the chrome toggling, gesture classification,
the keyboard guards, the toolbar's contents, the boundary messages — is pinned
in MobileReaderView.test.ts, mobileReaderGestures.test.ts, ReaderView.test.ts
and useReaderPresentation.test.ts, where none of it needs a browser. The CSS it
asserted (the font-size hop, the code block's touch-action, the 44px controls)
is in frontend/scripts/check-style-contracts.mjs; the geometry it asserted is in
the manual table in docs/development/testing-levels.md.
*/
test('drives the mobile reader with real touch gestures', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${baseUrl}/books`);
  await importEpub(page);
  const reader = await openReader(page);
  // The reader opened in its own tab; give it the mobile viewport this test
  // exercises (the popup inherited the context's desktop default).
  await reader.setViewportSize({ width: 390, height: 844 });

  const mobileReader = reader.locator('[data-reader-variant="mobile"]');
  await expect(mobileReader).toBeVisible();

  const box = await mobileReader.boundingBox();
  expect(box).not.toBeNull();
  const center = { x: box!.x + box!.width / 2, y: box!.y + box!.height / 2 };

  // A tap in the centre band is the only way to the chrome, and the chapter
  // counter in it is how the rest of this case reads the reader's position.
  await dispatchTouch(mobileReader, [center, center]);
  await expect(mobileReader.locator('.mobile-reader-toolbar')).toBeVisible();
  await expect(mobileReader.getByText('1 / 3')).toBeVisible();
  await expect(mobileReader.getByText(epubFixtureDescription)).toBeVisible();

  // Right to left turns the page forward, and the chapter that arrives is the
  // one the importer converted out of the EPUB.
  await dispatchTouch(mobileReader, [
    { x: box!.x + box!.width * 0.8, y: center.y },
    { x: box!.x + box!.width * 0.5, y: center.y + 4 },
    { x: box!.x + box!.width * 0.2, y: center.y + 6 }
  ], 2);
  await expect(mobileReader.getByText('2 / 3')).toBeVisible();
  await expect(mobileReader.getByRole('heading', { name: epubFixtureChapters[0] })).toBeVisible();

  await dispatchTouch(mobileReader, [
    { x: box!.x + box!.width * 0.2, y: center.y },
    { x: box!.x + box!.width * 0.5, y: center.y },
    { x: box!.x + box!.width * 0.8, y: center.y }
  ], 3);
  await expect(mobileReader.getByText('1 / 3')).toBeVisible();
  await expect(mobileReader.getByText(epubFixtureDescription)).toBeVisible();
});
