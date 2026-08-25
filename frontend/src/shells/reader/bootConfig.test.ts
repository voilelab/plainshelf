/**
 * @vitest-environment jsdom
 */
import { afterEach, describe, expect, it } from 'vitest';

import { readerBootConfig } from './bootConfig';
import { activeReaderShelfInfo } from './shelfInfo';
import { isReaderRuntime } from '@/providers/runtime';

function setInjected(value: unknown): void {
  (window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__ = value;
}

afterEach(() => {
  delete (window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__;
});

describe('reader boot config', () => {
  it('reads the injected config', () => {
    setInjected({ shelf_id: 'book', book_id: 'dune1234' });

    expect(readerBootConfig()).toEqual({ shelf_id: 'book', book_id: 'dune1234', section: null });
    expect(isReaderRuntime()).toBe(true);
    expect(activeReaderShelfInfo()).toEqual({ id: 'book', name: 'Book' });
  });

  // A reader launched for a specific chapter carries its section; section 0 (the
  // first chapter) is a real request and must survive rather than read as absent.
  it('reads the injected launch section', () => {
    setInjected({ shelf_id: 'shelf-real', book_id: 'dune1234', section: 3 });
    expect(readerBootConfig()).toEqual({ shelf_id: 'shelf-real', book_id: 'dune1234', section: 3 });

    setInjected({ shelf_id: 'shelf-real', book_id: 'dune1234', section: 0 });
    expect(readerBootConfig()?.section).toBe(0);
  });

  // Anything that is not a whole number is no deep link at all, matching
  // parseSectionQuery, so the reader opens at the restored progress.
  it('treats a missing or non-integer section as no deep link', () => {
    setInjected({ shelf_id: 'book', book_id: 'dune1234', section: 2.5 });
    expect(readerBootConfig()?.section).toBeNull();

    setInjected({ shelf_id: 'book', book_id: 'dune1234', section: '3' });
    expect(readerBootConfig()?.section).toBeNull();
  });

  // Nothing injected means an ordinary web or desktop build, which must not
  // take the reader's shelf or its route policy.
  it('reports no config outside the reader app', () => {
    expect(readerBootConfig()).toBeNull();
    expect(isReaderRuntime()).toBe(false);
    expect(activeReaderShelfInfo()).toBeNull();
  });

  // The app injects the marker before the user has picked a folder, so "reader
  // with no book" has to be distinguishable from "not the reader".
  it('reports the reader runtime with no book open', () => {
    setInjected({ shelf_id: 'book', book_id: '' });

    expect(isReaderRuntime()).toBe(true);
    expect(readerBootConfig()).toEqual({ shelf_id: 'book', book_id: '', section: null });
    expect(activeReaderShelfInfo()).toEqual({ id: 'book', name: 'Book' });
  });

  it('ignores an injected value of the wrong shape', () => {
    setInjected({ shelf_id: 42 });

    expect(readerBootConfig()).toEqual({ shelf_id: '', book_id: '', section: null });
    expect(activeReaderShelfInfo()).toBeNull();
  });
});
