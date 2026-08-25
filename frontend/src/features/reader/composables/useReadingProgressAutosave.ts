import { ref } from 'vue';

export const READING_PROGRESS_AUTOSAVE_INTERVAL_MS = 10_000;

type SaveProgress = (bookID: string, offset: number, at: number) => Promise<void>;

interface BookProgressState {
  currentOffset: number;
  // The time currentOffset was set, captured when the position changes rather
  // than when the buffered save flushes. A save (and any retry of it) carries
  // this so a movement buffered for up to the autosave interval cannot flush
  // with a newer timestamp than a later movement from another process and roll
  // it back under the newest-wins merge.
  changedAt: number;
  // What the last completed save actually persisted. A snapshot is clean only
  // when both the offset and its timestamp still match these: a position that
  // moved away and back to the same offset while an older save was in flight has
  // the same offset but a newer changedAt, and must be re-saved so its newer time
  // — not a concurrent process's write from the interim — wins the merge.
  savedOffset: number;
  savedAt: number;
}

/**
 * Buffers the reader's current position in memory and persists only dirty
 * snapshots. Calls to flush are queued so a slow storage backend cannot let an
 * older write race a newer position.
 */
export function useReadingProgressAutosave(saveProgress: SaveProgress) {
  const saveError = ref('');
  const progressByBook = new Map<string, BookProgressState>();
  const errorsByBook = new Map<string, string>();
  let currentBookID = '';
  let intervalID: ReturnType<typeof setInterval> | null = null;
  let queue: Promise<void> = Promise.resolve();

  function refreshSaveError(): void {
    saveError.value = errorsByBook.values().next().value ?? '';
  }

  // Dirty when the current position or its change time differs from what the last
  // save persisted. Comparing the timestamp too is what catches an offset that
  // returned to a saved value with a newer change time.
  function isDirty(progress: BookProgressState): boolean {
    return progress.currentOffset !== progress.savedOffset || progress.changedAt !== progress.savedAt;
  }

  /**
   * Selects the book being read and records what storage returned for it. If a
   * previous save for this book failed, keep its newer in-memory position so a
   * route change cannot discard the dirty snapshot before it can be retried.
   */
  function setBaseline(bookID: string, offset: number): number {
    currentBookID = bookID;
    const existing = progressByBook.get(bookID);
    if (existing && isDirty(existing)) {
      return existing.currentOffset;
    }

    const now = Date.now();
    progressByBook.set(bookID, {
      currentOffset: offset,
      changedAt: now,
      savedOffset: offset,
      savedAt: now
    });
    errorsByBook.delete(bookID);
    refreshSaveError();
    return offset;
  }

  function update(offset: number): void {
    const progress = progressByBook.get(currentBookID);
    if (!progress || progress.currentOffset === offset) {
      return;
    }
    progress.currentOffset = offset;
    progress.changedAt = Date.now();
  }

  async function saveDirtySnapshots(): Promise<void> {
    for (const [bookID, progress] of progressByBook) {
      if (!isDirty(progress)) {
        continue;
      }

      const offset = progress.currentOffset;
      const at = progress.changedAt;
      try {
        await saveProgress(bookID, offset, at);
        // Mark clean against the snapshot that was actually persisted. If the
        // position moved during the await (including back to this same offset
        // with a newer time), it stays dirty and the next flush re-saves it.
        progress.savedOffset = offset;
        progress.savedAt = at;
        errorsByBook.delete(bookID);
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to save reading progress';
        errorsByBook.set(bookID, message);
        console.warn('Failed to save reading progress; it will be retried', error);
      }
    }
    refreshSaveError();
  }

  /**
   * Queue one dirty check behind any write already in flight. A caller that
   * needs to leave the reader can await this and know its snapshot was checked
   * after the earlier write completed.
   */
  function flush(): Promise<void> {
    const next = queue.then(saveDirtySnapshots);
    queue = next.catch(() => undefined);
    return next;
  }

  function onVisibilityChange(): void {
    if (document.hidden) {
      void flush();
    }
  }

  function onPageHide(): void {
    void flush();
  }

  function start(): void {
    if (intervalID !== null) {
      return;
    }

    intervalID = setInterval(() => void flush(), READING_PROGRESS_AUTOSAVE_INTERVAL_MS);
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', onVisibilityChange);
    }
    if (typeof window !== 'undefined') {
      window.addEventListener('pagehide', onPageHide);
    }
  }

  function stop(): Promise<void> {
    if (intervalID !== null) {
      clearInterval(intervalID);
      intervalID = null;
    }
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibilityChange);
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener('pagehide', onPageHide);
    }
    return flush();
  }

  return {
    saveError,
    setBaseline,
    update,
    flush,
    start,
    stop
  };
}
