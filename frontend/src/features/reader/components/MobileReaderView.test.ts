// @vitest-environment jsdom
import { createApp, h, nextTick, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MobileReaderView from './MobileReaderView.vue';
import type { ReaderSection } from '@/types/book';

// ReaderContent renders the book body through the API client and the block
// window; neither is under test here, so it is replaced by an empty stub.
vi.mock('@/features/reader/components/ReaderContent.vue', () => ({
  default: { name: 'ReaderContentStub', render: () => null }
}));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref } = await vi.importActual<typeof import('vue')>('vue');
  return { useWriteAccess: () => ({ writesEnabled: ref(true) }) };
});

const HINT_KEY = 'reader-mobile-gesture-hint-seen';
const HINT_TEXT = 'Tap the center for controls · Swipe left or right to change chapters';

function section(index: number, title: string): ReaderSection {
  return { index, startOffset: 0, endOffset: title.length, title, text: title };
}

let app: App | null = null;
let host: HTMLElement | null = null;

function mount(): HTMLElement {
  host = document.createElement('div');
  document.body.append(host);
  app = createApp({
    setup: () => () =>
      h(MobileReaderView, {
        bookId: 'book-1',
        title: 'Book',
        bookFormat: 'txt',
        sourceId: 'source-1',
        sections: [section(0, 'A'), section(1, 'B')],
        currentSectionIndex: 0,
        currentSection: section(0, 'A'),
        progressPercent: 0,
        loading: false,
        error: '',
        saveError: '',
        showBackNavigation: false,
        isAtMinFontSize: false,
        isAtMaxFontSize: false
      })
  });
  app.mount(host);
  return host;
}

// jsdom never fires transitionend, so a leaving <Transition> child stays in the
// DOM forever; a node in its leave phase counts as dismissed.
function hint(root: HTMLElement): HTMLElement | null {
  return root.querySelector('.mobile-reader-hint:not(.reader-message-leave-active)');
}

function helpButton(root: HTMLElement): HTMLButtonElement {
  const button = root.querySelector<HTMLButtonElement>('.mobile-reader-help-tool');
  if (!button) throw new Error('help button is not rendered');
  return button;
}

// jsdom has no PointerEvent; the component only reads the pointer fields it
// adds here, so a MouseEvent carrying them drives the same code path.
function pointerEvent(type: string): MouseEvent {
  const event = new MouseEvent(type, { clientX: 200, clientY: 400, button: 0 });
  return Object.assign(event, { pointerId: 1, pointerType: 'mouse', isPrimary: true });
}

/** Opens the chrome the way a centre tap does, so the toolbar is in the DOM. */
async function openChrome(root: HTMLElement): Promise<void> {
  const page = root.querySelector<HTMLElement>('.mobile-reader-page');
  if (!page) throw new Error('reader page is not rendered');
  page.getBoundingClientRect = () =>
    ({ left: 0, top: 0, width: 400, height: 800 }) as DOMRect;
  page.dispatchEvent(pointerEvent('pointerdown'));
  page.dispatchEvent(pointerEvent('pointerup'));
  await nextTick();
}

beforeEach(() => {
  vi.useFakeTimers();
  window.localStorage.clear();
});

afterEach(() => {
  app?.unmount();
  host?.remove();
  app = null;
  host = null;
  vi.useRealTimers();
});

describe('MobileReaderView gesture hint', () => {
  it('shows the hint on the first open and marks it seen', async () => {
    const root = mount();
    await nextTick();

    expect(hint(root)?.textContent?.trim()).toBe(HINT_TEXT);
    expect(window.localStorage.getItem(HINT_KEY)).toBe('1');
  });

  it('does not show the hint automatically once it has been seen', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await nextTick();

    expect(hint(root)).toBeNull();
  });

  it('recalls the hint from the toolbar after it has been seen', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await openChrome(root);

    helpButton(root).click();
    await nextTick();

    expect(hint(root)?.textContent?.trim()).toBe(HINT_TEXT);
    expect(hint(root)?.classList.contains('mobile-reader-hint-above-chrome')).toBe(true);
  });

  it('dismisses a recalled hint on the same timer as the first one', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await openChrome(root);

    helpButton(root).click();
    await nextTick();
    vi.advanceTimersByTime(3_900);
    await nextTick();
    expect(hint(root)).not.toBeNull();

    vi.advanceTimersByTime(200);
    await nextTick();
    expect(hint(root)).toBeNull();
  });

  it('restarts the timer instead of stacking one per press', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await openChrome(root);

    helpButton(root).click();
    await nextTick();
    vi.advanceTimersByTime(3_000);
    helpButton(root).click();
    await nextTick();

    vi.advanceTimersByTime(1_100);
    await nextTick();
    expect(hint(root)).not.toBeNull();

    vi.advanceTimersByTime(3_000);
    await nextTick();
    expect(hint(root)).toBeNull();
  });

  it('keeps every toolbar control reachable beside the new button', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await openChrome(root);

    const labels = [...root.querySelectorAll('.mobile-reader-toolbar .mobile-reader-tool')].map(
      (button) => button.getAttribute('aria-label')
    );
    expect(labels).toEqual([
      'Decrease font size',
      'Increase font size',
      'Choose reading font',
      'Show chapters',
      'Show reading gestures'
    ]);
  });
});
