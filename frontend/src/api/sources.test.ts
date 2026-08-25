import { beforeEach, describe, expect, it, vi } from 'vitest';

const { fetchJsonMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn()
}));

vi.mock('./client', async () => {
  const actual = await vi.importActual<typeof import('./client')>('./client');
  return {
    ...actual,
    fetchJson: fetchJsonMock,
    buildShelfApiPath: (path: string) => path,
    isMockApiMode: () => false
  };
});

const { createSource } = await import('./sources');

describe('createSource', () => {
  beforeEach(() => {
    fetchJsonMock.mockReset().mockResolvedValue({
      schema_version: 1,
      id: 'derived-1',
      created_at: '2026-08-22T00:00:00Z',
      comment: 'converted',
      md5_hash: 'abc',
      format: 'md'
    });
  });

  it('sends a derived source as JSON instead of multipart', async () => {
    await createSource('book/one', {
      content: '# Book\n\n## 第一章\nBody 🍥',
      format: 'md',
      comment: 'converted',
      setCurrent: true
    });

    expect(fetchJsonMock).toHaveBeenCalledOnce();
    const [path, init, options] = fetchJsonMock.mock.calls[0];
    expect(path).toBe('/books/book%2Fone/sources');
    expect(init).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    expect(JSON.parse(init.body as string)).toEqual({
      content: '# Book\n\n## 第一章\nBody 🍥',
      format: 'md',
      comment: 'converted',
      set_current: true
    });
    expect(options).toEqual({ timeoutMs: 300_000 });
  });

  it('keeps an empty-source creation bodyless', async () => {
    await createSource('book-one');

    expect(fetchJsonMock).toHaveBeenCalledWith(
      '/books/book-one/sources',
      { method: 'POST', body: undefined },
      { timeoutMs: 300_000 }
    );
  });
});
