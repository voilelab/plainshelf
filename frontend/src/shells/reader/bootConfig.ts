interface ReaderBootConfig {
  shelf_id: string;
  book_id: string;
  section: number | null;
}

interface ReaderWindow extends Window {
  __PLAINSHELF_READER__?: Partial<ReaderBootConfig>;
}

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
    section:
      typeof injected.section === 'number' && Number.isInteger(injected.section)
        ? injected.section
        : null
  };
}
