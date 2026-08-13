import { ref } from 'vue';

export const READING_PROGRESS_AUTOSAVE_INTERVAL_MS = 10_000;

type SaveProgress = (bookID: string, offset: number) => Promise<void>;

interface ProgressSnapshot {
  bookID: string;
  offset: number;
}

/**
 * Buffers the reader's current position in memory and persists only dirty
 * snapshots. Calls to flush are queued so a slow storage backend cannot let an
 * older write race a newer position.
 */
export function useReadingProgressAutosave(saveProgress: SaveProgress) {
  const saveError = ref('');
  let current: ProgressSnapshot | null = null;
  let saved: ProgressSnapshot | null = null;
  let intervalID: ReturnType<typeof setInterval> | null = null;
  let queue: Promise<void> = Promise.resolve();

  function setBaseline(bookID: string, offset: number): void {
    current = { bookID, offset };
    saved = { bookID, offset };
    saveError.value = '';
  }

  function update(offset: number): void {
    if (!current || current.offset === offset) {
      return;
    }
    current = { ...current, offset };
  }

  function isDirty(): boolean {
    return Boolean(
      current &&
        (!saved || current.bookID !== saved.bookID || current.offset !== saved.offset)
    );
  }

  async function saveLatestSnapshot(): Promise<void> {
    if (!current || !isDirty()) {
      return;
    }

    const snapshot = { ...current };
    try {
      await saveProgress(snapshot.bookID, snapshot.offset);
      saved = snapshot;
      saveError.value = '';
    } catch (error) {
      saveError.value = error instanceof Error ? error.message : 'Failed to save reading progress';
      console.warn('Failed to save reading progress; it will be retried', error);
    }
  }

  /**
   * Queue one dirty check behind any write already in flight. A caller that
   * needs to leave the reader can await this and know its snapshot was checked
   * after the earlier write completed.
   */
  function flush(): Promise<void> {
    const next = queue.then(saveLatestSnapshot);
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
