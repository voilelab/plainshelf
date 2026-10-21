import { ref } from 'vue';

export const READING_PROGRESS_AUTOSAVE_INTERVAL_MS = 10_000;

type SaveProgress = (bookID: string, offset: number, at: number) => Promise<void>;

type StageProgress = (bookID: string, offset: number, at: number) => void;

interface BookProgressState {
  currentOffset: number;
  changedAt: number;
  savedOffset: number;
  savedAt: number;
}

export function useReadingProgressAutosave(saveProgress: SaveProgress, stageProgress?: StageProgress) {
  const saveError = ref('');
  const progressByBook = new Map<string, BookProgressState>();
  const errorsByBook = new Map<string, string>();
  let currentBookID = '';
  let intervalID: ReturnType<typeof setInterval> | null = null;
  let queue: Promise<void> = Promise.resolve();

  function refreshSaveError(): void {
    saveError.value = errorsByBook.values().next().value ?? '';
  }

  function isDirty(progress: BookProgressState): boolean {
    return progress.currentOffset !== progress.savedOffset || progress.changedAt !== progress.savedAt;
  }

  function selectBookPreservingUnsavedOffset(bookID: string, offset: number): number {
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
    stageProgress?.(currentBookID, offset, progress.changedAt);
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
    setBaseline: selectBookPreservingUnsavedOffset,
    update,
    flush,
    start,
    stop
  };
}
