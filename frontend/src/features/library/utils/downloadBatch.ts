/**
 * The decisions a batch "Download to device" makes, separated from the page's
 * progress and dialog refs so they can be checked without a device.
 *
 * Two of them are easy to get wrong and were only ever pinned end-to-end: an
 * already-downloaded book must not be fetched again (the button is offered on a
 * mixed selection, and re-downloading is the slow, data-costly mistake), and a
 * partly-failed run must leave exactly the failed books selected, so retrying is
 * one more tap rather than a re-selection.
 */
import type { Book } from '@/types/book';

interface DownloadBatchFailure {
  readonly id: string;
  readonly title: string;
}

interface DownloadBatchOutcome {
  /** Books that ended up downloaded, including those that already were. */
  readonly succeeded: number;
  readonly failures: readonly DownloadBatchFailure[];
  /** Ids actually handed to the provider, in order — nothing re-downloaded. */
  readonly requested: readonly string[];
}

/**
 * Downloads each book that is not already current, reporting progress as a
 * percentage after every book. One book's failure never stops the rest: a
 * selection of twenty must not be abandoned because the third one is corrupt.
 *
 * `onFailure` fires as each one happens rather than only at the end, so the
 * dialog's list fills in while the run is still going.
 */
export async function runDownloadBatch(
  books: readonly Book[],
  downloadBook: (id: string) => Promise<void>,
  callbacks: {
    onProgress?: (percentage: number) => void;
    onFailure?: (failure: DownloadBatchFailure) => void;
  } = {}
): Promise<DownloadBatchOutcome> {
  const failures: DownloadBatchFailure[] = [];
  const requested: string[] = [];
  let succeeded = 0;

  for (const [index, book] of books.entries()) {
    try {
      if (book.download_state !== 'downloaded') {
        requested.push(book.id);
        await downloadBook(book.id);
      }
      succeeded += 1;
    } catch {
      const failure = { id: book.id, title: book.title };
      failures.push(failure);
      callbacks.onFailure?.(failure);
    }
    callbacks.onProgress?.(((index + 1) / books.length) * 100);
  }

  return { succeeded, failures, requested };
}

/**
 * The selection to leave behind once the run finishes: the books that failed and
 * are still on the page, so the retry acts on what the user can see. An empty
 * set means clear the selection — everything worked, or the failures scrolled
 * out of the current page.
 */
export function retrySelection(
  failures: readonly DownloadBatchFailure[],
  visibleBookIds: readonly string[]
): Set<string> {
  const visible = new Set(visibleBookIds);
  return new Set(failures.map((failure) => failure.id).filter((id) => visible.has(id)));
}
