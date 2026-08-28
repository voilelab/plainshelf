// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';

vi.mock('@/layouts/MainLayout.vue', () => ({ default: {} }));
vi.mock('@/layouts/ReaderLayout.vue', () => ({ default: {} }));

const { default: router } = await import('./router');

// The standalone metadata editor page is gone and the editor opens as a modal
// from the book's own detail page. `/books/:id/edit` is deliberately not kept
// as a redirect: routing it back to the detail page had to carry a
// write-opening query through the redirect, which the mobile read-only guard
// and the detail page then both had to reason about. An old bookmark gets the
// not-found page instead, and this pins that it is not silently absorbed by
// the `books/:id` route or by any later child route.
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
