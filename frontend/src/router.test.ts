// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';

vi.mock('@/layouts/MainLayout.vue', () => ({ default: {} }));
vi.mock('@/layouts/ReaderLayout.vue', () => ({ default: {} }));

const { default: router } = await import('./router');

describe('the removed metadata editor route', () => {
  it('leaves a legacy /books/:id/edit URL at the not-found page', () => {
    expect(router.resolve('/books/book-1/edit').name).toBe('not-found');
  });

  it('still resolves the book detail route it was removed in favour of', () => {
    const detail = router.resolve('/books/book-1');
    expect(detail.name).toBe('book-detail');
    expect(detail.params).toEqual({ id: 'book-1' });
  });
});
