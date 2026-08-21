/**
 * What the reader app injects into index.html before the app's own scripts run.
 *
 * The desktop reader holds exactly one book, chosen with a native folder
 * dialog. Handing that book to the frontend in the page itself — rather than
 * through a request the shell would have to await — is what lets the first
 * paint already be the reader instead of an empty library.
 */
export interface ReaderBootConfig {
  /** The synthetic shelf the reader app reports; see readerapi.ShelfID. */
  shelf_id: string;
  /** The open book, or "" while the user has not picked a folder yet. */
  book_id: string;
}

interface ReaderWindow extends Window {
  __PLAINSHELF_READER__?: Partial<ReaderBootConfig>;
}

/**
 * The injected config, or null when this is not the reader app.
 *
 * Read per call rather than latched: nothing rewrites it in place — the app
 * reloads the window when a book is opened — so there is no stale value to
 * guard against, and a latch would only hide that from a test.
 */
export function readerBootConfig(): ReaderBootConfig | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const injected = (window as ReaderWindow).__PLAINSHELF_READER__;
  if (!injected) {
    return null;
  }

  return {
    shelf_id: typeof injected.shelf_id === 'string' ? injected.shelf_id : '',
    book_id: typeof injected.book_id === 'string' ? injected.book_id : ''
  };
}
