import { beforeEach, describe, expect, it } from 'vitest';

import { useToasts } from './useToasts';

function clearToasts(): void {
  const { toasts, dismissToast } = useToasts();
  for (const toast of [...toasts.value]) {
    dismissToast(toast.id);
  }
}

beforeEach(() => {
  clearToasts();
});

describe('useToasts', () => {
  it('shares one queue with every caller', () => {
    useToasts().showToast('first');
    useToasts().showToast('second');

    expect(useToasts().toasts.value.map((toast) => toast.message)).toEqual(['first', 'second']);
  });

  // Two rescans in a row must both be readable; the host renders a list, so a
  // second message may not silently replace the first.
  it('keeps identical messages apart', () => {
    const { showToast, toasts } = useToasts();

    const first = showToast('Found 3183 books in 21 folders');
    const second = showToast('Found 3183 books in 21 folders');

    expect(first).not.toBe(second);
    expect(toasts.value).toHaveLength(2);
  });

  it('dismisses only the toast asked for', () => {
    const { showToast, dismissToast, toasts } = useToasts();
    const first = showToast('first');
    showToast('second');

    dismissToast(first);

    expect(toasts.value.map((toast) => toast.message)).toEqual(['second']);
    // Dismissing a toast that has already gone is a no-op, not an error: the
    // timer and the close button can both fire for the same toast.
    dismissToast(first);
    expect(toasts.value).toHaveLength(1);
  });

  it('defaults to the neutral variant and takes an explicit one', () => {
    const { showToast, toasts } = useToasts();

    showToast('plain');
    showToast('bad', { variant: 'danger' });

    expect(toasts.value.map((toast) => toast.variant)).toEqual(['info', 'danger']);
  });
});
