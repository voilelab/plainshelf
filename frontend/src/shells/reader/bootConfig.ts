/**
 * What the reader app injects into index.html before the app's own scripts run.
 *
 * The desktop reader holds exactly one book, chosen with a native folder
 * dialog. Handing that book to the frontend in the page itself — rather than
 * through a request the shell would have to await — is what lets the first
 * paint already be the reader instead of an empty library.
 */
export interface ReaderBootConfig {
  /**
   * The shelf the reader app reports as active. It is the book's real desktop
   * shelf id when the desktop app launched the reader (so the reader addresses
   * the book, and reads its progress, under the desktop library's own shelf),
   * and the synthetic readerapi.ShelfID otherwise.
   */
  shelf_id: string;
  /** The open book, or "" while the user has not picked a folder yet. */
  book_id: string;
  /**
   * The reader section index to open at — the same value the `?section=` deep
   * link carries — when the reader was launched for a specific chapter (desktop
   * -section). null means no chapter was requested, so the reader opens at the
   * restored progress. The app injects it only when set, so an absent value
   * reads back as null.
   */
  section: number | null;
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
    book_id: typeof injected.book_id === 'string' ? injected.book_id : '',
    // Only a whole number is a section; anything else (absent, null, a stray
    // non-integer) means no deep link, matching parseSectionQuery's own rule.
    section:
      typeof injected.section === 'number' && Number.isInteger(injected.section)
        ? injected.section
        : null
  };
}
