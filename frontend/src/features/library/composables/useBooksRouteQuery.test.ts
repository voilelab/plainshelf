import { beforeEach, describe, expect, it, vi } from 'vitest';
import { reactive } from 'vue';
import type { LocationQuery } from 'vue-router';

const route = reactive<{ query: LocationQuery }>({ query: {} });
const push = vi.fn();
const replace = vi.fn();

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push, replace })
}));

const { useBooksRouteQuery } = await import('./useBooksRouteQuery');

function setQuery(query: LocationQuery): void {
  route.query = query;
}

beforeEach(() => {
  push.mockReset();
  replace.mockReset();
  setQuery({});
});

describe('charCountRange', () => {
  it('is empty when neither bound is in the URL', () => {
    const { charCountRange } = useBooksRouteQuery();
    expect(charCountRange.value).toEqual({ min: undefined, max: undefined });
  });

  it('reads each bound independently', () => {
    setQuery({ minChars: '100' });
    expect(useBooksRouteQuery().charCountRange.value).toEqual({ min: 100, max: undefined });

    setQuery({ maxChars: '900' });
    expect(useBooksRouteQuery().charCountRange.value).toEqual({ min: undefined, max: 900 });

    setQuery({ minChars: '100', maxChars: '900' });
    expect(useBooksRouteQuery().charCountRange.value).toEqual({ min: 100, max: 900 });
  });

  it('ignores an unusable bound rather than the whole range', () => {
    setQuery({ minChars: 'abc', maxChars: '900' });
    expect(useBooksRouteQuery().charCountRange.value).toEqual({ min: undefined, max: 900 });
  });
});

describe('buildBooksQuery char-count handling', () => {
  it('carries the current bounds through when charCount is omitted', () => {
    setQuery({ minChars: '100', maxChars: '900', page: '2' });
    const { pushBooksQuery } = useBooksRouteQuery();

    pushBooksQuery({ page: 3 });

    expect(push).toHaveBeenCalledWith({
      path: '/books',
      query: expect.objectContaining({ minChars: '100', maxChars: '900', page: '3' })
    });
  });

  it('writes new bounds and drops the ones left unset', () => {
    setQuery({ minChars: '100', maxChars: '900' });
    const { replaceBooksQuery } = useBooksRouteQuery();

    replaceBooksQuery({ page: 1, charCount: { min: 250 } });

    const query = replace.mock.calls[0][0].query;
    expect(query.minChars).toBe('250');
    expect(query).not.toHaveProperty('maxChars');
  });

  it('clears both bounds when handed an empty range', () => {
    setQuery({ minChars: '100', maxChars: '900' });
    const { replaceBooksQuery } = useBooksRouteQuery();

    replaceBooksQuery({ page: 1, charCount: {} });

    const query = replace.mock.calls[0][0].query;
    expect(query).not.toHaveProperty('minChars');
    expect(query).not.toHaveProperty('maxChars');
  });

  it('keeps a zero bound, which is a real filter and not an empty one', () => {
    const { replaceBooksQuery } = useBooksRouteQuery();

    replaceBooksQuery({ page: 1, charCount: { min: 0, max: 0 } });

    const query = replace.mock.calls[0][0].query;
    expect(query.minChars).toBe('0');
    expect(query.maxChars).toBe('0');
  });
});

describe('isBooksQueryNormalized', () => {
  it('accepts a URL that already carries the char-count bounds', () => {
    setQuery({ page: '1', sort: 'updated_at', order: 'desc', minChars: '100' });
    const { isBooksQueryNormalized } = useBooksRouteQuery();

    expect(isBooksQueryNormalized({ page: 1 })).toBe(true);
  });

  it('rejects a URL whose bounds differ from the requested ones', () => {
    setQuery({ page: '1', sort: 'updated_at', order: 'desc', minChars: '100' });
    const { isBooksQueryNormalized } = useBooksRouteQuery();

    expect(isBooksQueryNormalized({ page: 1, charCount: {} })).toBe(false);
  });
});
