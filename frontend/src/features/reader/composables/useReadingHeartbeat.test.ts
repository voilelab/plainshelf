import { effectScope, nextTick, ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { reportReadingActivityMock } = vi.hoisted(() => ({
  reportReadingActivityMock: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    reportReadingActivity: reportReadingActivityMock
  })
}));

const { useReadingHeartbeat } = await import('./useReadingHeartbeat');

// Tests run in a node environment; the composable only needs the listener
// registration surface and the `hidden` flag from `document`.
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
    reportReadingActivityMock.mockReset();
    reportReadingActivityMock.mockResolvedValue(undefined);
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

    expect(reportReadingActivityMock).toHaveBeenCalledTimes(1);
    expect(reportReadingActivityMock).toHaveBeenCalledWith('book-a', 45, expect.any(String));

    heartbeat.stop();
    scope.stop();
  });

  it('attributes the pre-change stretch to the old book when the reader switches books', async () => {
    const id = ref('book-a');
    const { scope, heartbeat } = setupHeartbeat(() => id.value);

    heartbeat.start();

    // Read book A for 20s, then navigate /reader/book-a -> /reader/book-b
    // (Vue Router reuses the ReaderView component, only the id changes).
    vi.advanceTimersByTime(20_000);
    id.value = 'book-b';
    await nextTick();

    expect(reportReadingActivityMock).toHaveBeenCalledTimes(1);
    expect(reportReadingActivityMock.mock.calls[0][0]).toBe('book-a');
    expect(reportReadingActivityMock.mock.calls[0][1]).toBe(20);

    // The next interval tick fires 45s after start; only the 25s that passed
    // since the switch belong to book B.
    vi.advanceTimersByTime(25_000);

    expect(reportReadingActivityMock).toHaveBeenCalledTimes(2);
    expect(reportReadingActivityMock.mock.calls[1][0]).toBe('book-b');
    expect(reportReadingActivityMock.mock.calls[1][1]).toBe(25);

    heartbeat.stop();
    scope.stop();
  });

  it('flushes the remainder on stop under the book that was being read', () => {
    const id = ref('book-a');
    const { scope, heartbeat } = setupHeartbeat(() => id.value);

    heartbeat.start();
    vi.advanceTimersByTime(10_000);
    heartbeat.stop();

    expect(reportReadingActivityMock).toHaveBeenCalledTimes(1);
    expect(reportReadingActivityMock).toHaveBeenCalledWith('book-a', 10, expect.any(String));

    scope.stop();
  });
});
