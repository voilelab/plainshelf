import { computed, effectScope, ref } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';
import { useMetadataEditorModal } from './useMetadataEditorModal';

function book(id: string, title = `Book ${id}`): Book {
  return { id, title, authors: [], tags: [], folders: [] };
}

describe('useMetadataEditorModal', () => {
  it('keeps a stable selected id until close and rejects unknown or competing selections', () => {
    const books = ref([book('book-1'), book('book-2')]);
    const readOnly = ref(false);
    const scope = effectScope();
    const editor = scope.run(() => useMetadataEditorModal({
      books,
      readOnly: computed(() => readOnly.value),
      refresh: vi.fn()
    }))!;

    editor.openEditor('missing');
    expect(editor.open.value).toBe(false);

    editor.openEditor('book-1');
    editor.openEditor('book-2');
    expect(editor.selectedBookId.value).toBe('book-1');

    // A folder/page refresh may replace the list while the modal is open. The
    // explicit id must not silently follow whichever card now occupies a slot.
    books.value = [book('book-2')];
    expect(editor.selectedBookId.value).toBe('book-1');
    expect(editor.open.value).toBe(true);

    editor.closeEditor();
    expect(editor.selectedBookId.value).toBeNull();
    expect(editor.open.value).toBe(false);
    scope.stop();
  });

  it('hard-blocks edit events while the page is read-only', () => {
    const books = ref([book('book-1')]);
    const scope = effectScope();
    const editor = scope.run(() => useMetadataEditorModal({
      books,
      readOnly: computed(() => true),
      refresh: vi.fn()
    }))!;

    editor.openEditor('book-1');
    expect(editor.selectedBookId.value).toBeNull();
    expect(editor.open.value).toBe(false);
    scope.stop();
  });

  it('patches the current list before refreshing after save', () => {
    const books = ref([book('book-1', 'Stale title'), book('book-2')]);
    const refresh = vi.fn(() => {
      expect(books.value[0].title).toBe('Fresh title');
    });
    const scope = effectScope();
    const editor = scope.run(() => useMetadataEditorModal({
      books,
      readOnly: computed(() => false),
      refresh
    }))!;

    editor.onSaved(book('book-1', 'Fresh title'));

    expect(books.value.map((value) => value.title)).toEqual(['Fresh title', 'Book book-2']);
    expect(refresh).toHaveBeenCalledTimes(1);
    scope.stop();
  });
});
