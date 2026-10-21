import { effectScope, nextTick, ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { addReadingSecondsMock } = vi.hoisted(() => ({
  addReadingSecondsMock: vi.fn()
}));

vi.mock('@/storage/readingStats', () => ({
  addReadingSeconds: addReadingSecondsMock
}));

const { useReadingHeartbeat } = await import('./useReadingHeartbeat');

function stubDocument(): void {
  vi.stubGlobal('document', {
    hidden: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn()
  });
}

function setupHeartbeat(id: () => string) {
  const scope = effectScope();
  let heartbeat!: ReturnType<typeof useReadingHeartbeat>;
  scope.run(() => {
    heartbeat = useReadingHeartbeat(id);
  });
  return { scope, heartbeat };
}

describe('useReadingHeartbeat', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    stubDocument();
    addReadingSecondsMock.mockReset();
    addReadingSecondsMock.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('reports each full tick under the current book id', () => {
    const id = ref('book-a');
    const { scope, heartbeat } = setupHeartbeat(() => id.value);

    heartbeat.start();
    vi.advanceTimersByTime(45_000);

    expect(addReadingSecondsMock).toHaveBeenCalledTimes(1);
    expect(addReadingSecondsMock).toHaveBeenCalledWith('book-a', 45, expect.any(String));

    heartbeat.stop();
    scope.stop();
  });

  it.each([[60_000, 60], [90_000, 68]])(
    'reports a delayed %i ms tick with a %i second cap',
    (elapsedMs, reportedSeconds) => {
      const { scope, heartbeat } = setupHeartbeat(() => 'book-a');
      heartbeat.start();
      vi.setSystemTime(Date.now() + elapsedMs - 45_000);
      vi.advanceTimersByTime(45_000);

      expect(addReadingSecondsMock).toHaveBeenCalledTimes(1);
      expect(addReadingSecondsMock).toHaveBeenCalledWith('book-a', reportedSeconds, expect.any(String));

      heartbeat.stop();
      scope.stop();
    }
  );

  it('attributes the pre-change stretch to the old book when the reader switches books', async () => {
    const id = ref('book-a');
    const { scope, heartbeat } = setupHeartbeat(() => id.value);

    heartbeat.start();

    vi.advanceTimersByTime(20_000);
    id.value = 'book-b';
    await nextTick();

    expect(addReadingSecondsMock).toHaveBeenCalledTimes(1);
    expect(addReadingSecondsMock.mock.calls[0][0]).toBe('book-a');
    expect(addReadingSecondsMock.mock.calls[0][1]).toBe(20);

    vi.advanceTimersByTime(25_000);

    expect(addReadingSecondsMock).toHaveBeenCalledTimes(2);
    expect(addReadingSecondsMock.mock.calls[1][0]).toBe('book-b');
    expect(addReadingSecondsMock.mock.calls[1][1]).toBe(25);

    heartbeat.stop();
    scope.stop();
  });

  it('flushes the remainder on stop under the book that was being read', () => {
    const id = ref('book-a');
    const { scope, heartbeat } = setupHeartbeat(() => id.value);

    heartbeat.start();
    vi.advanceTimersByTime(10_000);
    heartbeat.stop();

    expect(addReadingSecondsMock).toHaveBeenCalledTimes(1);
    expect(addReadingSecondsMock).toHaveBeenCalledWith('book-a', 10, expect.any(String));

    scope.stop();
  });
});
