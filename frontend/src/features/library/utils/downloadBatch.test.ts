import { describe, expect, it, vi } from 'vitest';
import { retrySelection, runDownloadBatch } from './downloadBatch';
import type { Book, DownloadState } from '@/types/book';

// The mobile long-press end-to-end case ended by proving three things about a
// batch download: a partly-failed run reports both tallies and keeps the failed
// book selected, a retry then succeeds, and selecting books that are already
// current does not call the provider again. Those are decisions, not device
// behavior, so they moved here when that case was removed.

function book(id: string, state?: DownloadState): Book {
  return { id, title: id.toUpperCase(), download_state: state } as Book;
}

describe('runDownloadBatch', () => {
  it('downloads every book that is not already current', async () => {
    const downloaded: string[] = [];
    const outcome = await runDownloadBatch(
      [book('a'), book('b')],
      async (id) => void downloaded.push(id)
    );

    expect(downloaded).toEqual(['a', 'b']);
    expect(outcome).toMatchObject({ succeeded: 2, failures: [] });
  });

  it('skips a book that is already downloaded, but still counts it a success', async () => {
    // Offered on a mixed selection, so re-fetching is the costly mistake — and
    // reporting the skipped book as a failure would be a lie.
    const download = vi.fn(async () => {});
    const outcome = await runDownloadBatch(
      [book('a', 'downloaded'), book('b', 'not_downloaded')],
      download
    );

    expect(outcome.requested).toEqual(['b']);
    expect(download).toHaveBeenCalledTimes(1);
    expect(outcome.succeeded).toBe(2);
  });

  it('re-fetches a book whose source moved on, rather than treating it as current', async () => {
    const outcome = await runDownloadBatch([book('a', 'update_available')], async () => {});

    expect(outcome.requested).toEqual(['a']);
  });

  it('carries on past a failure and names the book that failed', async () => {
    const outcome = await runDownloadBatch([book('a'), book('b'), book('c')], async (id) => {
      if (id === 'b') throw new Error('connection refused');
    });

    // The third book is not abandoned because the second one failed.
    expect(outcome.succeeded).toBe(2);
    expect(outcome.failures).toEqual([{ id: 'b', title: 'B' }]);
  });

  it('reports progress once per book, ending at 100', async () => {
    const progress: number[] = [];
    await runDownloadBatch(
      [book('a'), book('b'), book('c'), book('d')],
      async () => {},
      { onProgress: (percentage) => progress.push(percentage) }
    );

    expect(progress).toEqual([25, 50, 75, 100]);
  });

  it('reports progress for a failed book too, so the bar cannot stall', async () => {
    const progress: number[] = [];
    await runDownloadBatch(
      [book('a'), book('b')],
      async (id) => {
        if (id === 'a') throw new Error('boom');
      },
      { onProgress: (percentage) => progress.push(percentage) }
    );

    expect(progress).toEqual([50, 100]);
  });

  it('reports each failure as it happens, so the dialog fills in during the run', async () => {
    const seen: string[] = [];
    await runDownloadBatch(
      [book('a'), book('b'), book('c')],
      async (id) => {
        if (id !== 'b') throw new Error('boom');
      },
      { onFailure: (failure) => seen.push(failure.id) }
    );

    expect(seen).toEqual(['a', 'c']);
  });
});

describe('retrySelection', () => {
  it('keeps the failed books selected, so retrying is one more tap', () => {
    expect(retrySelection([{ id: 'b', title: 'B' }], ['a', 'b', 'c'])).toEqual(new Set(['b']));
  });

  it('clears the selection when everything worked', () => {
    expect(retrySelection([], ['a', 'b'])).toEqual(new Set());
  });

  it('drops a failure that is no longer on the page', () => {
    // Re-selecting a row the user cannot see would make the toolbar act on
    // something invisible.
    expect(retrySelection([{ id: 'z', title: 'Z' }], ['a', 'b'])).toEqual(new Set());
  });
});
