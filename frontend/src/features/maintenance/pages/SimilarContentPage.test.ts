// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { SimilarBookPair, SimilarRelation } from '@/api/books';

import { ref } from 'vue';

// Hoisted so the module mocks below can close over the same spies.
// `writesEnabled` is a real ref (assigned by the useWriteAccess mock) so the
// template unwraps it.
const mocks = vi.hoisted(() => ({
  getSimilarBookPairs: vi.fn(),
  getFingerprintStatus: vi.fn(),
  listBooks: vi.fn(),
  listSources: vi.fn(),
  startFingerprintSources: vi.fn(),
  deleteBook: vi.fn(),
  getTaskChain: vi.fn(),
  writesEnabled: { value: true }
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    getSimilarBookPairs: mocks.getSimilarBookPairs,
    getFingerprintStatus: mocks.getFingerprintStatus,
    listBooks: mocks.listBooks,
    listSources: mocks.listSources
  }),
  bookshelfWriter: () => ({
    startFingerprintSources: mocks.startFingerprintSources,
    deleteBook: mocks.deleteBook,
    getTaskChain: mocks.getTaskChain
  })
}));

// The sweep writes to the shelf, so the page asks the shared write gate — which
// covers a read-only shelf as well as a read-only server.
vi.mock('@/composables/useWriteAccess', () => {
  const writesEnabled = ref(true);
  mocks.writesEnabled = writesEnabled;
  return { useWriteAccess: () => ({ writesEnabled }) };
});

import SimilarContentPage from './SimilarContentPage.vue';
import { setLocale } from '@/i18n';
import { FingerprintSweepBusyError, SimilarTooLargeError } from '@/api/books';

function pair(jaccard: number, relation: SimilarRelation = 'near_identical'): SimilarBookPair {
  return {
    a: `a-${jaccard}-${relation}`,
    b: `b-${jaccard}-${relation}`,
    jaccard,
    containment_a: jaccard,
    containment_b: jaccard,
    norm_chars_a: 1000,
    norm_chars_b: 1000,
    relation
  };
}

// One representative per tier, plus the truncated-25% subset the toggle targets.
const truncatedSubset = pair(0.758, 'subset');
const allPairs: SimilarBookPair[] = [
  pair(0.999, 'identical_after_normalize'),
  pair(0.856),
  truncatedSubset,
  pair(0.501, 'same_source'),
  pair(0.292, 'same_source')
];

const NO_MISSING = { total: 10, fingerprinted: 10, missing: 0, algo: { normalize: '', shingle: '', hash: '', k: 0 } };

let mounted: { app: App; host: HTMLElement } | null = null;

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(SimilarContentPage) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function rows(host: HTMLElement): HTMLElement[] {
  return [...host.querySelectorAll<HTMLElement>('.similar-pair-card')];
}

function buttonByText(host: HTMLElement, text: string): HTMLButtonElement {
  const button = [...host.querySelectorAll('button')].find((el) => el.textContent?.trim() === text);
  if (!button) throw new Error(`no button with text "${text}"`);
  return button as HTMLButtonElement;
}

// The confirmation dialog is portalled to document.body (reka DialogPortal), so
// its buttons live outside the page host.
function dialogButtonByText(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find((el) => el.textContent?.trim() === text);
  if (!button) throw new Error(`no dialog button with text "${text}"`);
  return button as HTMLButtonElement;
}

beforeEach(() => {
  setLocale('en');
  mocks.getSimilarBookPairs.mockReset().mockResolvedValue(allPairs);
  mocks.getFingerprintStatus.mockReset().mockResolvedValue(NO_MISSING);
  // The card join and per-card source lookups are best-effort; empty results
  // keep these page-level tests focused on filtering, not card content.
  mocks.listBooks.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 0 });
  mocks.listSources.mockReset().mockResolvedValue([]);
  mocks.startFingerprintSources.mockReset().mockResolvedValue('chain-1');
  mocks.deleteBook.mockReset().mockResolvedValue(undefined);
  mocks.getTaskChain.mockReset();
  mocks.writesEnabled.value = true;
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('SimilarContentPage', () => {
  it('fetches similar pairs once, at the widest floor, and lists the default tier', async () => {
    const host = mount();
    await flush();

    expect(mocks.getSimilarBookPairs).toHaveBeenCalledTimes(1);
    expect(mocks.getSimilarBookPairs).toHaveBeenCalledWith(0.15);

    // Default tier is "same book" (J >= 0.45): the 0.292 pair is filtered out.
    expect(rows(host)).toHaveLength(4);
  });

  it('widens the tier without a second API call and result count grows', async () => {
    const host = mount();
    await flush();
    const before = rows(host).length;

    buttonByText(host, 'Possibly same source').click();
    await flush();

    expect(rows(host).length).toBeGreaterThan(before);
    expect(rows(host)).toHaveLength(5);
    expect(mocks.getSimilarBookPairs).toHaveBeenCalledTimes(1);
  });

  it('subset toggle is independent of the tier: the truncated copy shows at the strictest tier', async () => {
    const host = mount();
    await flush();

    // Strictest tier hides the 0.758 subset pair entirely.
    buttonByText(host, 'Nearly identical').click();
    await flush();
    expect(host.textContent).not.toContain(truncatedSubset.a);

    const checkbox = host.querySelector<HTMLInputElement>('.similarity-subset input');
    checkbox!.checked = true;
    checkbox!.dispatchEvent(new Event('change'));
    await flush();

    const shown = rows(host);
    expect(shown).toHaveLength(1);
    expect(host.textContent).toContain(truncatedSubset.a);
    expect(mocks.getSimilarBookPairs).toHaveBeenCalledTimes(1);
  });

  it('keeps the bar with only a force button when nothing is missing', async () => {
    const host = mount();
    await flush();

    const bar = host.querySelector('.similar-fingerprint-bar');
    expect(bar).not.toBeNull();
    expect(bar!.textContent).toContain('All 10 books are fingerprinted');
    // The incremental build button is gone once nothing is missing, but force
    // stays: a stat comparison can miss a change that leaves nothing "missing".
    expect(
      [...host.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Build fingerprints')
    ).toBe(false);
    expect(buttonByText(host, 'Force rebuild')).toBeTruthy();
  });

  it('hides the whole bar when the shelf has no books', async () => {
    mocks.getFingerprintStatus.mockResolvedValue({ ...NO_MISSING, total: 0, fingerprinted: 0 });
    const host = mount();
    await flush();
    expect(host.querySelector('.similar-fingerprint-bar')).toBeNull();
  });

  it('offers both a build and a force button when fingerprints are missing on a writable shelf', async () => {
    mocks.getFingerprintStatus.mockResolvedValue({ ...NO_MISSING, missing: 3 });
    const host = mount();
    await flush();

    const bar = host.querySelector('.similar-fingerprint-bar');
    expect(bar).not.toBeNull();
    expect(bar!.textContent).toContain('3 of 10 books have no fingerprint yet');
    expect(buttonByText(host, 'Build fingerprints')).toBeTruthy();
    expect(buttonByText(host, 'Force rebuild')).toBeTruthy();
  });

  it('force rebuild asks for confirmation before it runs anything', async () => {
    const host = mount();
    await flush();

    buttonByText(host, 'Force rebuild').click();
    await flush();

    // The click only opened the dialog; the destructive-feeling sweep has not
    // started, so a stray click costs nothing.
    expect(document.body.textContent).toContain('Rebuild every fingerprint?');
    expect(mocks.startFingerprintSources).not.toHaveBeenCalled();

    // Cancelling dismisses it and still runs nothing.
    dialogButtonByText('Cancel').click();
    await flush();
    expect(mocks.startFingerprintSources).not.toHaveBeenCalled();
  });

  it('confirming the dialog schedules a forced sweep, ignoring the cache', async () => {
    const host = mount();
    await flush();

    buttonByText(host, 'Force rebuild').click();
    await flush();
    dialogButtonByText('Rebuild all').click();
    await flush();

    expect(mocks.startFingerprintSources).toHaveBeenCalledTimes(1);
    expect(mocks.startFingerprintSources).toHaveBeenCalledWith(true);
  });

  it('a busy sweep after confirming shows a retryable notice, not "rebuilding"', async () => {
    mocks.startFingerprintSources.mockReset().mockRejectedValueOnce(new FingerprintSweepBusyError());
    const host = mount();
    await flush();

    buttonByText(host, 'Force rebuild').click();
    await flush();
    dialogButtonByText('Rebuild all').click();
    await flush();

    expect(mocks.startFingerprintSources).toHaveBeenCalledWith(true);
    // A benign notice, and crucially no chain is polled and no "Rebuilding…"
    // label — the forced rebuild did not happen and must not claim it did.
    expect(host.querySelector('.similar-fingerprint-note')?.textContent).toContain('already running');
    expect(mocks.getTaskChain).not.toHaveBeenCalled();
    expect([...host.querySelectorAll('button')].some((b) => b.textContent?.includes('Rebuilding'))).toBe(false);
  });

  it('a read-only shelf explains it cannot build and hides both buttons', async () => {
    mocks.getFingerprintStatus.mockResolvedValue({ ...NO_MISSING, missing: 3 });
    mocks.writesEnabled.value = false;
    const host = mount();
    await flush();

    expect(host.querySelector('.similar-fingerprint-readonly')?.textContent).toContain(
      'read-only shelf cannot build fingerprints'
    );
    const labels = [...host.querySelectorAll('button')].map((b) => b.textContent?.trim());
    expect(labels).not.toContain('Build fingerprints');
    expect(labels).not.toContain('Force rebuild');
    expect(mocks.startFingerprintSources).not.toHaveBeenCalled();
  });

  it('shows the empty state when no pair meets the filter', async () => {
    mocks.getSimilarBookPairs.mockResolvedValue([]);
    const host = mount();
    await flush();
    expect(host.querySelector('.similar-empty')).not.toBeNull();
    expect(rows(host)).toHaveLength(0);
  });

  it('shows the over-budget estimate and runs a confirmed comparison on request', async () => {
    mocks.getSimilarBookPairs
      .mockRejectedValueOnce(new SimilarTooLargeError(2_000_000_000, 1_073_741_824, 12, 10, 45, 60))
      .mockResolvedValueOnce(allPairs);
    const host = mount();
    await flush();

    expect(host.querySelector('.similar-error')).toBeNull();
    const notice = host.querySelector('.similar-notice');
    expect(notice).not.toBeNull();
    expect(notice!.textContent).toContain('10 of 12 books');
    expect(notice!.textContent).toContain('45 comparisons');
    expect(notice!.textContent).toContain('2,000,000,000 merge steps');
    expect(notice!.textContent).toContain('60 seconds');

    buttonByText(host, 'Compare anyway').click();
    await flush();

    expect(mocks.getSimilarBookPairs).toHaveBeenNthCalledWith(2, 0.15, true);
    expect(host.querySelector('.similar-notice')).toBeNull();
    expect(rows(host).length).toBeGreaterThan(0);
  });

  it('shows an error with retry when the comparison fails', async () => {
    mocks.getSimilarBookPairs.mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(allPairs);
    const host = mount();
    await flush();

    const errorBox = host.querySelector('.similar-error');
    expect(errorBox).not.toBeNull();
    expect(errorBox!.textContent).toContain('boom');

    buttonByText(host, 'Retry').click();
    await flush();

    expect(host.querySelector('.similar-error')).toBeNull();
    expect(rows(host).length).toBeGreaterThan(0);
    expect(mocks.getSimilarBookPairs).toHaveBeenCalledTimes(2);
  });
});
