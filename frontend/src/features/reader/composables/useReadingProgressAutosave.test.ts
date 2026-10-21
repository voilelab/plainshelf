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
    vi.setSystemTime(0);
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
    expect(save).toHaveBeenCalledWith('book-a', 20, 0);
    await autosave.stop();
  });

  it('writes once across repeated flushes of an unchanged position', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.start();
    autosave.update(30);

    await autosave.flush();
    await autosave.flush();
    await autosave.flush();
    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS * 2);

    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith('book-a', 30, 0);
    await autosave.stop();
  });

  it('timestamps a position at change time, not flush time', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.start();
    autosave.update(10);

    await vi.advanceTimersByTimeAsync(READING_PROGRESS_AUTOSAVE_INTERVAL_MS);
    expect(save).toHaveBeenCalledWith('book-a', 10, 0);
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
      ['book-a', 10, 0],
      ['book-a', 20, 0]
    ]);
  });

  it('re-saves a position that returned to a saved offset with a newer timestamp', async () => {
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

    vi.setSystemTime(5);
    autosave.update(20);
    vi.setSystemTime(9);
    autosave.update(10);

    resolveFirst();
    await firstFlush;

    await autosave.flush();

    expect(save.mock.calls).toEqual([
      ['book-a', 10, 0],
      ['book-a', 10, 9]
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
    expect(save.mock.calls).toEqual([
      ['book-a', 10, 0],
      ['book-a', 10, 0]
    ]);
    expect(autosave.saveError.value).toBe('');
    await autosave.stop();
  });

  it('retains failed progress for an old book after switching books', async () => {
    const save = vi
      .fn()
      .mockRejectedValueOnce(new Error('disk unavailable'))
      .mockResolvedValue(undefined);
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.update(10);

    await autosave.flush();
    expect(autosave.saveError.value).toBe('disk unavailable');

    autosave.setBaseline('book-b', 0);
    autosave.update(20);
    await autosave.flush();

    expect(save.mock.calls).toEqual([
      ['book-a', 10, 0],
      ['book-a', 10, 0],
      ['book-b', 20, 0]
    ]);
    expect(autosave.saveError.value).toBe('');
  });

  it('restores an unsaved in-memory position when revisiting a book', async () => {
    const save = vi.fn().mockRejectedValue(new Error('disk unavailable'));
    const autosave = useReadingProgressAutosave(save);
    autosave.setBaseline('book-a', 0);
    autosave.update(10);
    await autosave.flush();

    autosave.setBaseline('book-b', 0);
    expect(autosave.setBaseline('book-a', 0)).toBe(10);
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
      ['book-a', 10, 0],
      ['book-b', 15, 0]
    ]);
  });

  it('stages each real position change immediately, but not an unchanged one', () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const stage = vi.fn();
    const autosave = useReadingProgressAutosave(save, stage);
    autosave.setBaseline('book-a', 0);

    expect(stage).not.toHaveBeenCalled();

    autosave.update(10);
    expect(stage).toHaveBeenCalledOnce();
    expect(stage).toHaveBeenCalledWith('book-a', 10, 0);

    autosave.update(10);
    expect(stage).toHaveBeenCalledOnce();

    autosave.update(25);
    expect(stage).toHaveBeenLastCalledWith('book-a', 25, 0);
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
      ['book-a', 10, 0],
      ['book-a', 20, 0],
      ['book-a', 30, 0]
    ]);
    expect(documentTarget.removeEventListener).toHaveBeenCalledWith(
      'visibilitychange',
      expect.any(Function)
    );
    expect(windowTarget.removeEventListener).toHaveBeenCalledWith('pagehide', expect.any(Function));
  });
});
