import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  READING_PROGRESS_AUTOSAVE_INTERVAL_MS,
  useReadingProgressAutosave
} from './useReadingProgressAutosave';

interface EventTargetStub {
  addEventListener: ReturnType<typeof vi.fn>;
  removeEventListener: ReturnType<typeof vi.fn>;
  listeners: Map<string, () => void>;
}

function eventTargetStub(): EventTargetStub {
  const listeners = new Map<string, () => void>();
  return {
    listeners,
    addEventListener: vi.fn((name: string, listener: () => void) => listeners.set(name, listener)),
    removeEventListener: vi.fn((name: string) => listeners.delete(name))
  };
}

describe('useReadingProgressAutosave', () => {
  let documentTarget: EventTargetStub & { hidden: boolean };
  let windowTarget: EventTargetStub;

  beforeEach(() => {
    vi.useFakeTimers();
    documentTarget = { ...eventTargetStub(), hidden: false };
    windowTarget = eventTargetStub();
    vi.stubGlobal('document', documentTarget);
    vi.stubGlobal('window', windowTarget);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('does not write an unchanged baseline', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 12);
    autosave.start();

    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS * 3);

    expect(save).not.toHaveBeenCalled();
    await autosave.stop();
  });

  it('writes only the latest dirty position on the next interval', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.start();
    autosave.update(10);
    autosave.update(20);

    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS);
    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS);

    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith('book-a', 20);
    await autosave.stop();
  });

  it('queues a newer position behind a write already in flight', async () => {
    let resolveFirst!: () => void;
    const firstWrite = new Promise<void>((resolve) => {
      resolveFirst = resolve;
    });
    const save = vi.fn().mockReturnValueOnce(firstWrite).mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.update(10);

    const firstFlush = autosave.flush();
    await Promise.resolve();
    autosave.update(20);
    const finalFlush = autosave.flush();
    resolveFirst();
    await Promise.all([firstFlush, finalFlush]);

    expect(save.mock.calls).toEqual([
      ['book-a', 10],
      ['book-a', 20]
    ]);
  });

  it('retains a failed snapshot for retry and clears the warning after success', async () => {
    const save = vi
      .fn()
      .mockRejectedValueOnce(new Error('disk full'))
      .mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.update(10);
    autosave.start();

    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS);
    expect(autosave.saveError.value).toBe('disk full');

    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS);
    expect(save).toHaveBeenCalledTimes(2);
    expect(autosave.saveError.value).toBe('');
    await autosave.stop();
  });

  it('latches each book id before switching baselines', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.update(10);
    await autosave.flush();
    autosave.setBaseline('book-b', 5);
    autosave.update(15);
    await autosave.flush();

    expect(save.mock.calls).toEqual([
      ['book-a', 10],
      ['book-b', 15]
    ]);
  });

  it('flushes when hidden or page-hidden and removes lifecycle listeners on stop', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.start();

    autosave.update(10);
    documentTarget.hidden = true;
    documentTarget.listeners.get('visibilitychange')?.();
    await autosave.flush();

    autosave.update(20);
    windowTarget.listeners.get('pagehide')?.();
    await autosave.flush();

    autosave.update(30);
    await autosave.stop();
    autosave.update(40);
    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS * 2);

    expect(save.mock.calls).toEqual([
      ['book-a', 10],
      ['book-a', 20],
      ['book-a', 30]
    ]);
    expect(documentTarget.removeEventListener).toHaveBeenCalledWith(
      'visibilitychange',
      expect.any(Function)
    );
    expect(windowTarget.removeEventListener).toHaveBeenCalledWith('pagehide', expect.any(Function));
  });
});
