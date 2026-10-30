import type { Book } from '@/types/book';

interface DownloadBatchFailure {
  readonly id: string;
  readonly title: string;
}

interface DownloadBatchOutcome {
  readonly succeeded: number;
  readonly failures: readonly DownloadBatchFailure[];
  readonly requested: readonly string[];
}

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

export function retrySelection(
  failures: readonly DownloadBatchFailure[],
  visibleBookIds: readonly string[]
): Set<string> {
  const visible = new Set(visibleBookIds);
  return new Set(failures.map((failure) => failure.id).filter((id) => visible.has(id)));
}
