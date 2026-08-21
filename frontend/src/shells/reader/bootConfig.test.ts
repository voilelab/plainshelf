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

    expect(readerBootConfig()).toEqual({ shelf_id: 'book', book_id: 'dune1234' });
    expect(isReaderRuntime()).toBe(true);
    expect(activeReaderShelfInfo()).toEqual({ id: 'book', name: 'Book' });
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
    expect(readerBootConfig()).toEqual({ shelf_id: 'book', book_id: '' });
    expect(activeReaderShelfInfo()).toEqual({ id: 'book', name: 'Book' });
  });

  it('ignores an injected value of the wrong shape', () => {
    setInjected({ shelf_id: 42 });

    expect(readerBootConfig()).toEqual({ shelf_id: '', book_id: '' });
    expect(activeReaderShelfInfo()).toBeNull();
  });
});
