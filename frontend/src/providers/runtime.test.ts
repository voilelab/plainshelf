/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { isNativePlatform } = vi.hoisted(() => ({ isNativePlatform: vi.fn() }));

vi.mock('@capacitor/core', () => ({
  Capacitor: { isNativePlatform }
}));

async function loadRuntime(url: string, native: boolean) {
  vi.resetModules();
  isNativePlatform.mockReturnValue(native);
  window.history.replaceState({}, '', url);
  return import('./runtime');
}

describe('isMobileRuntime / isMobileShellPreview', () => {
  beforeEach(() => {
    isNativePlatform.mockReset();
  });

  it('treats a native shell as a mobile runtime but not a preview', async () => {
    const runtime = await loadRuntime('/books', true);

    expect(runtime.isMobileRuntime()).toBe(true);
    expect(runtime.isMobileShellPreview()).toBe(false);
  });

  it('treats the browser preview as both', async () => {
    const runtime = await loadRuntime('/books?mobile-shell-preview=1', false);

    expect(runtime.isMobileRuntime()).toBe(true);
    expect(runtime.isMobileShellPreview()).toBe(true);
  });

  it('treats a plain web page as neither', async () => {
    const runtime = await loadRuntime('/books', false);

    expect(runtime.isMobileRuntime()).toBe(false);
    expect(runtime.isMobileShellPreview()).toBe(false);
  });

  it('latches the preview flag so an in-app navigation cannot drop it', async () => {
    const runtime = await loadRuntime('/books?mobile-shell-preview=1', false);
    expect(runtime.isMobileShellPreview()).toBe(true);

    window.history.replaceState({}, '', '/books');

    expect(runtime.isMobileShellPreview()).toBe(true);
    expect(runtime.isMobileRuntime()).toBe(true);
  });
});

describe('isWebRuntime', () => {
  beforeEach(() => {
    isNativePlatform.mockReset();
  });

  it('is true for a plain web page', async () => {
    const runtime = await loadRuntime('/books', false);

    expect(runtime.isWebRuntime()).toBe(true);
  });

  it('is false inside a native mobile shell', async () => {
    const runtime = await loadRuntime('/books', true);

    expect(runtime.isWebRuntime()).toBe(false);
  });

  it('is false under the mobile shell preview', async () => {
    const runtime = await loadRuntime('/books?mobile-shell-preview=1', false);

    expect(runtime.isWebRuntime()).toBe(false);
  });

  it('is false under the desktop shell preview', async () => {
    const runtime = await loadRuntime('/books?desktop-shell-preview=1', false);

    expect(runtime.isWebRuntime()).toBe(false);
  });

  it('is false for the standalone reader app', async () => {
    const runtime = await loadRuntime('/books', false);
    (window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__ = {};

    try {
      expect(runtime.isWebRuntime()).toBe(false);
    } finally {
      delete (window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__;
    }
  });
});
