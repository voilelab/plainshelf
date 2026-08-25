/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  resolve: vi.fn((to: unknown) => ({
    href:
      typeof to === 'string'
        ? to
        : `${(to as { path: string }).path}${
            (to as { query?: { section?: string } }).query?.section !== undefined
              ? `?section=${(to as { query: { section: string } }).query.section}`
              : ''
          }`
  })),
  showToast: vi.fn(),
  isWebRuntime: vi.fn(() => false),
  // The device-local reader-launch preference launchReader reads at click time.
  // Defaults to 'new-reader' so the pre-existing cases assert that behaviour.
  getReaderLaunchMode: vi.fn(() => 'new-reader' as 'new-reader' | 'in-window'),
  // undefined models a provider without the desktop reader (web/mobile); a
  // spy models the desktop provider.
  openDesktopReader: undefined as undefined | ((bookId: string, section?: number) => Promise<void>)
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push, resolve: mocks.resolve })
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () =>
    mocks.openDesktopReader ? { openDesktopReader: mocks.openDesktopReader } : {},
  isWebRuntime: mocks.isWebRuntime
}));

vi.mock('@/composables/useReaderLaunchPreference', () => ({
  getReaderLaunchMode: mocks.getReaderLaunchMode
}));

vi.mock('@/composables/useToasts', () => ({
  useToasts: () => ({ showToast: mocks.showToast })
}));

vi.mock('@/i18n', () => ({ t: (key: string) => key }));

import { useReaderLaunch } from './useReaderLaunch';

describe('useReaderLaunch launchReader', () => {
  let openSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.isWebRuntime.mockReturnValue(false);
    mocks.getReaderLaunchMode.mockReturnValue('new-reader');
    mocks.openDesktopReader = undefined;
    // window.open is unimplemented in jsdom; a spy both silences it and lets us
    // assert the new-tab call. Default to a truthy handle (pop-up allowed).
    openSpy = vi.spyOn(window, 'open').mockReturnValue({} as Window);
  });

  it('opens the reader in a new tab on a web build instead of navigating in place', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    expect(openSpy).toHaveBeenCalledWith('/reader/book-1', '_blank', 'noopener,noreferrer');
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('carries the section index into the new-tab URL on a web build', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    const { launchReader } = useReaderLaunch();

    launchReader('book-1', 3);

    expect(openSpy).toHaveBeenCalledWith('/reader/book-1?section=3', '_blank', 'noopener,noreferrer');
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('navigates in place via router.push on a non-web build', () => {
    mocks.isWebRuntime.mockReturnValue(false);
    const { launchReader } = useReaderLaunch();

    launchReader('book-1', 3);

    expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1', query: { section: '3' } });
    expect(openSpy).not.toHaveBeenCalled();
  });

  it('opens the standalone reader on the desktop app instead of navigating in place', () => {
    const openReader = vi.fn().mockResolvedValue(undefined);
    mocks.openDesktopReader = openReader;
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    // No section requested: the reader opens at the restored progress.
    expect(openReader).toHaveBeenCalledWith('book-1', undefined);
    expect(mocks.push).not.toHaveBeenCalled();
    expect(openSpy).not.toHaveBeenCalled();
    // A successful launch stays quiet — the toast is only for the fallback.
    expect(mocks.showToast).not.toHaveBeenCalled();
  });

  it('falls back to the in-app reader when the standalone reader will not launch', async () => {
    const openReader = vi.fn().mockRejectedValue(new Error('not installed'));
    mocks.openDesktopReader = openReader;
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    await vi.waitFor(() => expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1' }));
    expect(openReader).toHaveBeenCalledWith('book-1', undefined);
    // The user picked "open a new reader", so the fallback is no longer silent:
    // an error without the unsupported-platform code reads as a macOS launch
    // failure (reader not installed / launch errored).
    expect(mocks.showToast).toHaveBeenCalledWith('bookDetail.messages.readerLaunchFailed');
  });

  it('explains the platform when the standalone reader is unsupported here', async () => {
    // The desktop backend embeds this code in its non-macOS OpenReader error;
    // the real matcher in @/api/desktop keys the message off it.
    const openReader = vi
      .fn()
      .mockRejectedValue(
        new Error(
          'main.(*DesktopApp).OpenReader: reader_unsupported_platform: opening the standalone reader is only supported on macOS'
        )
      );
    mocks.openDesktopReader = openReader;
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    await vi.waitFor(() => expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1' }));
    expect(mocks.showToast).toHaveBeenCalledWith('bookDetail.messages.readerUnsupportedPlatform');
  });

  it('explains the platform when Wails rejects with a bare error string', async () => {
    // Wails rejects a bound-method promise with the Go error's message string,
    // not an Error, so the real desktop path never carries an Error instance;
    // the classifier must still recognise the unsupported-platform code.
    const openReader = vi
      .fn()
      .mockRejectedValue(
        'main.(*DesktopApp).OpenReader: reader_unsupported_platform: opening the standalone reader is only supported on macOS'
      );
    mocks.openDesktopReader = openReader;
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    await vi.waitFor(() => expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1' }));
    expect(mocks.showToast).toHaveBeenCalledWith('bookDetail.messages.readerUnsupportedPlatform');
  });

  it('opens the standalone reader at the chapter on a desktop chapter jump', () => {
    const openReader = vi.fn().mockResolvedValue(undefined);
    mocks.openDesktopReader = openReader;
    const { launchReader } = useReaderLaunch();

    launchReader('book-1', 3);

    // A chapter jump now shells out to the standalone reader too, passing the
    // section so it opens on that chapter rather than being kept in-app.
    expect(openReader).toHaveBeenCalledWith('book-1', 3);
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('falls back to the in-app reader at the chapter when a chapter jump will not launch', async () => {
    const openReader = vi.fn().mockRejectedValue(new Error('not installed'));
    mocks.openDesktopReader = openReader;
    const { launchReader } = useReaderLaunch();

    launchReader('book-1', 3);

    await vi.waitFor(() =>
      expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1', query: { section: '3' } })
    );
    expect(openReader).toHaveBeenCalledWith('book-1', 3);
    expect(mocks.showToast).toHaveBeenCalledWith('bookDetail.messages.readerLaunchFailed');
  });

  // noopener/noreferrer make window.open return null even on success, so the web
  // path must never touch router.push — doing so would navigate the original tab
  // on every successful open, defeating the new-tab behaviour. openSpy returns
  // null here to model that spec behaviour.
  it('does not navigate the original tab on a web build even when window.open returns null', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    openSpy.mockReturnValue(null);
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    expect(openSpy).toHaveBeenCalledWith('/reader/book-1', '_blank', 'noopener,noreferrer');
    expect(mocks.push).not.toHaveBeenCalled();
  });

  // With the 'in-window' preference the web build navigates in place instead of
  // opening a new tab, and must not touch window.open at all.
  it('navigates in place on a web build when the preference is in-window', () => {
    mocks.isWebRuntime.mockReturnValue(true);
    mocks.getReaderLaunchMode.mockReturnValue('in-window');
    const { launchReader } = useReaderLaunch();

    launchReader('book-1', 3);

    expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1', query: { section: '3' } });
    expect(openSpy).not.toHaveBeenCalled();
  });

  // With the 'in-window' preference the desktop build navigates in place instead
  // of shelling out, and must not call openDesktopReader at all.
  it('navigates in place on the desktop app when the preference is in-window', () => {
    const openReader = vi.fn().mockResolvedValue(undefined);
    mocks.openDesktopReader = openReader;
    mocks.getReaderLaunchMode.mockReturnValue('in-window');
    const { launchReader } = useReaderLaunch();

    launchReader('book-1');

    expect(mocks.push).toHaveBeenCalledWith({ path: '/reader/book-1' });
    expect(openReader).not.toHaveBeenCalled();
    expect(openSpy).not.toHaveBeenCalled();
    // Reverse condition: skipping the shell-out must not raise a fallback toast.
    expect(mocks.showToast).not.toHaveBeenCalled();
  });
});

describe('useReaderLaunch onReaderLinkClick', () => {
  let openSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.isWebRuntime.mockReturnValue(true);
    mocks.getReaderLaunchMode.mockReturnValue('new-reader');
    mocks.openDesktopReader = undefined;
    openSpy = vi.spyOn(window, 'open').mockReturnValue({} as Window);
  });

  function clickEvent(init: MouseEventInit = {}): MouseEvent {
    // button/metaKey/etc. are read-only on a constructed MouseEvent, so they
    // must be seeded through the init dict, not assigned after. cancelable makes
    // preventDefault() observable; button 0 with no modifiers is a plain primary
    // click.
    return new MouseEvent('click', { button: 0, cancelable: true, ...init });
  }

  it('runs the launch preference and suppresses the default on a plain click', () => {
    const { onReaderLinkClick } = useReaderLaunch();
    const event = clickEvent();

    onReaderLinkClick(event, 'book-1');

    expect(event.defaultPrevented).toBe(true);
    expect(openSpy).toHaveBeenCalledWith('/reader/book-1', '_blank', 'noopener,noreferrer');
  });

  it.each([
    ['metaKey', { metaKey: true }],
    ['ctrlKey', { ctrlKey: true }],
    ['shiftKey', { shiftKey: true }],
    ['altKey', { altKey: true }]
  ])('leaves a %s-modified click to the browser default', (_label, mod) => {
    const { onReaderLinkClick } = useReaderLaunch();
    const event = clickEvent(mod);

    onReaderLinkClick(event, 'book-1');

    // The browser opens the real href its own way: no preventDefault, no launch.
    expect(event.defaultPrevented).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
    expect(mocks.push).not.toHaveBeenCalled();
  });

  it('leaves a non-primary button to the browser default', () => {
    const { onReaderLinkClick } = useReaderLaunch();
    const event = clickEvent({ button: 1 });

    onReaderLinkClick(event, 'book-1');

    expect(event.defaultPrevented).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
    expect(mocks.push).not.toHaveBeenCalled();
  });
});
