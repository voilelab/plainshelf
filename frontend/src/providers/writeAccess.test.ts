import { afterEach, describe, expect, it, vi } from 'vitest';

import type { BookshelfProvider, BookshelfWriter } from './bookshelfProvider';

const reader = { listBooks: vi.fn() } as unknown as BookshelfProvider;
const writer = { listBooks: vi.fn(), writable: true } as unknown as BookshelfProvider;

async function loadProviders(active: BookshelfProvider) {
  vi.resetModules();
  vi.doMock('./runtime', () => ({
    isMobileRuntime: () => false,
    isWailsRuntime: () => false
  }));
  vi.doMock('./serverBookshelfProvider', () => ({
    ServerBookshelfProvider: function ServerBookshelfProvider() {
      return active;
    }
  }));
  return import('./index');
}

afterEach(() => {
  vi.doUnmock('./runtime');
  vi.doUnmock('./serverBookshelfProvider');
  vi.resetModules();
});

describe('isWritableProvider', () => {
  it('accepts a provider that declares the write surface', async () => {
    const { isWritableProvider } = await loadProviders(writer);
    expect(isWritableProvider(writer)).toBe(true);
  });

  it('rejects one that does not', async () => {
    const { isWritableProvider } = await loadProviders(reader);
    expect(isWritableProvider(reader)).toBe(false);
  });

  it('rejects a provider carrying write methods but no marker', async () => {
    const { isWritableProvider } = await loadProviders(reader);
    const halfWriter = { listBooks: vi.fn(), updateBook: vi.fn() } as unknown as BookshelfProvider;
    expect(isWritableProvider(halfWriter)).toBe(false);
  });
});

describe('bookshelfWriter', () => {
  it('hands back the active provider when it can write', async () => {
    const { bookshelfWriter } = await loadProviders(writer);
    expect(bookshelfWriter()).toBe(writer);
  });

  it('refuses with a 403 ApiError when it cannot', async () => {
    const { bookshelfWriter } = await loadProviders(reader);

    expect(() => bookshelfWriter()).toThrow(/read-only/);
    try {
      bookshelfWriter();
      expect.unreachable('bookshelfWriter must refuse a read-only provider');
    } catch (err) {
      expect(err).toMatchObject({ name: 'ApiError', status: 403 });
    }
  });
});

describe('the write surface is optional on BookshelfProvider', () => {
  it('does not accept a plain provider as a writer', () => {
    // @ts-expect-error a BookshelfProvider is not known to implement BookshelfWriter
    const asWriter: BookshelfWriter = reader;
    expect(asWriter).toBeDefined();
  });
});
