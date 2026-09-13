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

/** Every chapter change the component asked its parent for, in order. */
const emitted: string[] = [];

function mount(currentSectionIndex = 0): HTMLElement {
  host = document.createElement('div');
  document.body.append(host);
  const sections = [section(0, 'A'), section(1, 'B')];
  app = createApp({
    setup: () => () =>
      h(MobileReaderView, {
        bookId: 'book-1',
        title: 'Book',
        bookFormat: 'txt',
        sourceId: 'source-1',
        sections,
        currentSectionIndex,
        currentSection: sections[currentSectionIndex],
        progressPercent: 0,
        loading: false,
        error: '',
        saveError: '',
        showBackNavigation: false,
        isAtMinFontSize: false,
        isAtMaxFontSize: false,
        onPreviousSection: () => emitted.push('previousSection'),
        onNextSection: () => emitted.push('nextSection')
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
function pointerEvent(type: string, clientX = 200, clientY = 400): MouseEvent {
  const event = new MouseEvent(type, { clientX, clientY, button: 0 });
  return Object.assign(event, { pointerId: 1, pointerType: 'mouse', isPrimary: true });
}

/** The reader page, with a bounding box so the centre band can be computed. */
function readerPage(root: HTMLElement): HTMLElement {
  const page = root.querySelector<HTMLElement>('.mobile-reader-page');
  if (!page) throw new Error('reader page is not rendered');
  page.getBoundingClientRect = () => ({ left: 0, top: 0, width: 400, height: 800 }) as DOMRect;
  return page;
}

/**
 * A horizontal drag past the swipe distance. Pointer input rather than touch:
 * the component hands physical touches to its TouchEvent path, and jsdom has no
 * constructible Touch either way. The classification itself is pinned in
 * mobileReaderGestures.test.ts; what these cases are about is what the
 * component does with the answer.
 */
async function swipe(root: HTMLElement, direction: 'next' | 'previous'): Promise<void> {
  const page = readerPage(root);
  const [from, to] = direction === 'next' ? [300, 180] : [180, 300];
  page.dispatchEvent(pointerEvent('pointerdown', from));
  page.dispatchEvent(pointerEvent('pointerup', to));
  await nextTick();
}

// Same leave-phase caveat as hint(): the chrome rides a <Transition>, and jsdom
// never fires transitionend, so a node on its way out stays in the DOM.
function toolbar(root: HTMLElement): HTMLElement | null {
  return root.querySelector('.mobile-reader-toolbar:not(.reader-chrome-leave-active)');
}

function boundaryMessage(root: HTMLElement): string {
  return root.querySelector('.mobile-reader-live')?.textContent?.trim() ?? '';
}

/** Opens the chrome the way a centre tap does, so the toolbar is in the DOM. */
async function openChrome(root: HTMLElement): Promise<void> {
  const page = readerPage(root);
  page.dispatchEvent(pointerEvent('pointerdown'));
  page.dispatchEvent(pointerEvent('pointerup'));
  await nextTick();
}

beforeEach(() => {
  vi.useFakeTimers();
  window.localStorage.clear();
  emitted.length = 0;
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

/*
The chrome is what an immersive reader hides, so when it appears and when it
stays is the whole contract. These were end-to-end assertions until the browser
turned out to add nothing to them but a four-second sleep.
*/
describe('MobileReaderView chrome', () => {
  it('stays out of the way until a centre tap asks for it', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await nextTick();
    expect(toolbar(root)).toBeNull();

    await openChrome(root);
    expect(toolbar(root)).not.toBeNull();
  });

  it('puts the chrome away again on the next tap', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await openChrome(root);
    expect(toolbar(root)).not.toBeNull();

    await openChrome(root);
    expect(toolbar(root)).toBeNull();
  });

  // The hint and the chrome are dismissed by different things — the hint by a
  // timer, the chrome only by a tap. Sharing the timer would take the toolbar
  // away four seconds after it was asked for. Recalling the hint from the
  // toolbar is the case that has both on screen with the timer running.
  it('keeps the chrome up when the hint timer fires under it', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();
    await openChrome(root);
    helpButton(root).click();
    await nextTick();
    expect(hint(root)).not.toBeNull();

    vi.advanceTimersByTime(4_100);
    await nextTick();

    expect(hint(root)).toBeNull();
    expect(toolbar(root)).not.toBeNull();
  });
});

/*
At either end of the book a swipe has nowhere to go. Saying so is better than
doing nothing, and it must not reach the parent as a chapter change the parent
would then have to ignore.
*/
describe('MobileReaderView chapter boundaries', () => {
  it('turns the page when there is a page to turn to', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount();

    await swipe(root, 'next');

    expect(emitted).toEqual(['nextSection']);
    expect(boundaryMessage(root)).toBe('');
  });

  it('says so instead of turning back past the first chapter', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount(0);

    await swipe(root, 'previous');

    expect(emitted).toEqual([]);
    expect(boundaryMessage(root)).toBe('You are at the first chapter');
  });

  it('says so instead of turning on past the last chapter', async () => {
    window.localStorage.setItem(HINT_KEY, '1');
    const root = mount(1);

    await swipe(root, 'next');

    expect(emitted).toEqual([]);
    expect(boundaryMessage(root)).toBe('You are at the last chapter');
  });
});
