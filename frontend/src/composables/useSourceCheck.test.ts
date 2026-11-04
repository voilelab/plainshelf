import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { effectScope, ref } from 'vue';
import { setLocale } from '@/i18n';
import { useSourceCheck } from './useSourceCheck';
import type { ShelfSourceKind } from '@/utils/shelfSourceCheck';

function setup(check: Parameters<typeof useSourceCheck>[1]) {
  const fields = {
    kind: ref<ShelfSourceKind>('plainshelf'),
    url: ref(''),
    username: ref(''),
    secret: ref('')
  };
  const scope = effectScope();
  const state = scope.run(() => useSourceCheck(fields, check))!;
  return { fields, state, scope };
}

beforeEach(() => {
  setLocale('en');
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useSourceCheck', () => {
  it('checks once the fields settle, not on every keystroke', async () => {
    const check = vi.fn(() => Promise.resolve({ books: 3 }));
    const { fields, state } = setup(check);

    fields.url.value = 'http://lan/dav/a/';
    fields.secret.value = 't';
    fields.secret.value = 'tok';
    await vi.runAllTimersAsync();

    expect(check).toHaveBeenCalledOnce();
    expect(check).toHaveBeenCalledWith({ source: 'plainshelf', url: 'http://lan/dav/a/', username: '', secret: 'tok' });
    expect(state.status.value).toBe('ok');
    expect(state.books.value).toBe(3);
  });

  it('waits for a token before checking a PlainShelf share', async () => {
    const check = vi.fn(() => Promise.resolve({ books: 0 }));
    const { fields, state } = setup(check);

    fields.url.value = 'http://lan/dav/a/';
    await vi.runAllTimersAsync();

    expect(check).not.toHaveBeenCalled();
    expect(state.ready.value).toBe(false);
  });

  it('drops an answer for values edited since', async () => {
    let answer: (value: { books: number }) => void = () => undefined;
    const check = vi.fn(() => new Promise<{ books: number }>((resolve) => (answer = resolve)));
    const { fields, state } = setup(check);
    fields.url.value = 'http://lan/dav/a/';
    fields.secret.value = 'tok';
    await vi.runAllTimersAsync();

    fields.url.value = 'http://lan/dav/b/';
    answer({ books: 2 });
    await Promise.resolve();

    expect(state.status.value).toBe('idle');
  });

  it('reports a refused token and checks again on demand', async () => {
    const check = vi.fn(() => Promise.resolve({ code: 'SOURCE_UNAUTHORIZED', books: 0 }));
    const { fields, state } = setup(check);
    fields.url.value = 'http://lan/dav/a/';
    fields.secret.value = 'tok';

    await expect(state.ensureChecked()).resolves.toBe(false);
    expect(state.status.value).toBe('error');
    expect(state.error.value).toContain('Wrong token');
  });

  it('reuses a passed check instead of asking again', async () => {
    const check = vi.fn(() => Promise.resolve({ books: 1 }));
    const { fields, state } = setup(check);
    fields.url.value = 'http://lan/dav/a/';
    fields.secret.value = 'tok';
    await vi.runAllTimersAsync();

    await expect(state.ensureChecked()).resolves.toBe(true);
    expect(check).toHaveBeenCalledOnce();
  });

  it('cancels a pending check when its scope ends', async () => {
    const check = vi.fn(() => Promise.resolve({ books: 1 }));
    const { fields, scope } = setup(check);
    fields.url.value = 'http://lan/dav/a/';
    fields.secret.value = 'tok';

    scope.stop();
    await vi.runAllTimersAsync();

    expect(check).not.toHaveBeenCalled();
  });
});
