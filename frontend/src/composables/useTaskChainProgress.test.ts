import { effectScope } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { TaskChain, TaskStatus } from '../types/task';

const { getTaskChainMock } = vi.hoisted(() => ({
  getTaskChainMock: vi.fn()
}));

vi.mock('../providers', () => ({
  getBookshelfProvider: () => ({
    getTaskChain: getTaskChainMock
  })
}));

const { useTaskChainProgress } = await import('./useTaskChainProgress');

function chain(status: TaskStatus, percentage: number): TaskChain {
  return {
    id: 'chain-1',
    name: 'empty_trash',
    title: 'Empty trash',
    status,
    percentage,
    tasks: []
  };
}

// flush lets pending promise callbacks run between fake-timer ticks.
function flush(): Promise<void> {
  return Promise.resolve().then(() => undefined);
}

describe('useTaskChainProgress', () => {
  beforeEach(() => {
    getTaskChainMock.mockReset();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('polls until the chain settles, then reports the final status', async () => {
    getTaskChainMock
      .mockResolvedValueOnce(chain('running', 50))
      .mockResolvedValueOnce(chain('completed', 100));

    const settled: TaskStatus[] = [];
    const scope = effectScope();
    const progress = scope.run(() =>
      useTaskChainProgress({
        intervalMs: 10,
        onSettled: (status) => {
          settled.push(status);
        }
      })
    )!;

    await progress.start(async () => 'chain-1');
    expect(progress.running.value).toBe(true);
    expect(progress.status.value).toBe('pending');

    await vi.advanceTimersByTimeAsync(10);
    await flush();
    expect(progress.status.value).toBe('running');
    expect(progress.percentage.value).toBe(50);
    expect(progress.finished.value).toBe(false);

    await vi.advanceTimersByTimeAsync(10);
    await flush();
    expect(progress.status.value).toBe('completed');
    expect(progress.percentage.value).toBe(100);
    expect(progress.finished.value).toBe(true);
    expect(progress.running.value).toBe(false);
    expect(settled).toEqual(['completed']);

    // No further polling once the chain has settled.
    await vi.advanceTimersByTimeAsync(100);
    expect(getTaskChainMock).toHaveBeenCalledTimes(2);

    scope.stop();
  });

  it('treats a partially completed chain as settled', async () => {
    getTaskChainMock.mockResolvedValueOnce(chain('partially_completed', 100));

    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    await progress.start(async () => 'chain-1');
    await vi.advanceTimersByTimeAsync(10);
    await flush();

    expect(progress.status.value).toBe('partially_completed');
    expect(progress.finished.value).toBe(true);
    expect(progress.running.value).toBe(false);

    scope.stop();
  });

  it('records the error and stops reporting itself busy when starting fails', async () => {
    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    await progress.start(async () => {
      throw new Error('worker is busy');
    });

    expect(progress.error.value).toBe('worker is busy');
    expect(progress.started.value).toBe(false);
    expect(progress.running.value).toBe(false);
    expect(getTaskChainMock).not.toHaveBeenCalled();

    scope.stop();
  });

  it('settles when polling fails so the caller can be dismissed', async () => {
    getTaskChainMock.mockRejectedValueOnce(new Error('network down'));

    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    await progress.start(async () => 'chain-1');
    await vi.advanceTimersByTimeAsync(10);
    await flush();

    expect(progress.error.value).toBe('network down');
    expect(progress.status.value).toBe('failed');
    expect(progress.finished.value).toBe(true);
    expect(progress.running.value).toBe(false);

    scope.stop();
  });

  // The work is already under way while the submission is in flight, so a
  // caller gating a modal on `running` must not be able to dismiss it or fire a
  // second submission during that window.
  it('reports itself running while the submission is still in flight', async () => {
    let resolveBegin: (id: string) => void = () => {};
    const begin = new Promise<string>((resolve) => {
      resolveBegin = resolve;
    });

    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    const pending = progress.start(() => begin);
    await flush();

    expect(progress.started.value).toBe(true);
    expect(progress.running.value).toBe(true);
    expect(progress.finished.value).toBe(false);

    // A second start during the window is refused.
    const secondBegin = vi.fn(async () => 'chain-2');
    await progress.start(secondBegin);
    expect(secondBegin).not.toHaveBeenCalled();

    getTaskChainMock.mockResolvedValue(chain('completed', 100));
    resolveBegin('chain-1');
    await pending;

    expect(progress.taskChainId.value).toBe('chain-1');

    scope.stop();
  });

  it('discards a submission that resolves after a reset', async () => {
    let resolveBegin: (id: string) => void = () => {};
    const begin = new Promise<string>((resolve) => {
      resolveBegin = resolve;
    });
    getTaskChainMock.mockResolvedValue(chain('running', 10));

    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    const pending = progress.start(() => begin);
    await flush();

    progress.reset();
    resolveBegin('chain-1');
    await pending;

    expect(progress.taskChainId.value).toBeNull();
    expect(progress.started.value).toBe(false);
    expect(progress.running.value).toBe(false);

    // No hidden polling for a chain the caller is no longer displaying.
    await vi.advanceTimersByTimeAsync(100);
    expect(getTaskChainMock).not.toHaveBeenCalled();

    scope.stop();
  });

  it('reset clears the state and cancels the pending poll', async () => {
    getTaskChainMock.mockResolvedValue(chain('running', 10));

    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    await progress.start(async () => 'chain-1');
    progress.reset();

    expect(progress.started.value).toBe(false);
    expect(progress.percentage.value).toBe(0);
    expect(progress.status.value).toBe('pending');

    await vi.advanceTimersByTimeAsync(100);
    expect(getTaskChainMock).not.toHaveBeenCalled();

    scope.stop();
  });

  it('stops polling when the owning scope is disposed', async () => {
    getTaskChainMock.mockResolvedValue(chain('running', 10));

    const scope = effectScope();
    const progress = scope.run(() => useTaskChainProgress({ intervalMs: 10 }))!;

    await progress.start(async () => 'chain-1');
    scope.stop();

    await vi.advanceTimersByTimeAsync(100);
    expect(getTaskChainMock).not.toHaveBeenCalled();
  });
});
