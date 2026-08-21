// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { SimilarBookPair, SimilarRelation } from '@/api/books';

import { ref } from 'vue';

// Hoisted so the module mocks below can close over the same spies. `readOnly`
// is a real ref (assigned by the useServerMode mock) so the template unwraps it.
const mocks = vi.hoisted(() => ({
  getSimilarBookPairs: vi.fn(),
  getFingerprintStatus: vi.fn(),
  startFingerprintSources: vi.fn(),
  getTaskChain: vi.fn(),
  readOnly: { value: false }
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    getSimilarBookPairs: mocks.getSimilarBookPairs,
    getFingerprintStatus: mocks.getFingerprintStatus
  }),
  bookshelfWriter: () => ({
    startFingerprintSources: mocks.startFingerprintSources,
    getTaskChain: mocks.getTaskChain
  })
}));

vi.mock('@/composables/useServerMode', () => {
  const readOnly = ref(false);
  mocks.readOnly = readOnly;
  return { useServerMode: () => ({ readOnly }) };
});

import SimilarContentPage from './SimilarContentPage.vue';
import { setLocale } from '@/i18n';
import { SimilarTooLargeError } from '@/api/books';

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
  return [...host.querySelectorAll<HTMLElement>('.similar-pair-row')];
}

function buttonByText(host: HTMLElement, text: string): HTMLButtonElement {
  const button = [...host.querySelectorAll('button')].find((el) => el.textContent?.trim() === text);
  if (!button) throw new Error(`no button with text "${text}"`);
  return button as HTMLButtonElement;
}

beforeEach(() => {
  setLocale('en');
  mocks.getSimilarBookPairs.mockReset().mockResolvedValue(allPairs);
  mocks.getFingerprintStatus.mockReset().mockResolvedValue(NO_MISSING);
  mocks.startFingerprintSources.mockReset().mockResolvedValue('chain-1');
  mocks.getTaskChain.mockReset();
  mocks.readOnly.value = false;
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

  it('hides the fingerprint bar when nothing is missing', async () => {
    const host = mount();
    await flush();
    expect(host.querySelector('.similar-fingerprint-bar')).toBeNull();
  });

  it('offers a build button when fingerprints are missing on a writable shelf', async () => {
    mocks.getFingerprintStatus.mockResolvedValue({ ...NO_MISSING, missing: 3 });
    const host = mount();
    await flush();

    const bar = host.querySelector('.similar-fingerprint-bar');
    expect(bar).not.toBeNull();
    expect(bar!.textContent).toContain('3 of 10 books have no fingerprint yet');
    expect(buttonByText(host, 'Build fingerprints')).toBeTruthy();
  });

  it('a read-only shelf explains it cannot build and hides the button', async () => {
    mocks.getFingerprintStatus.mockResolvedValue({ ...NO_MISSING, missing: 3 });
    mocks.readOnly.value = true;
    const host = mount();
    await flush();

    expect(host.querySelector('.similar-fingerprint-readonly')?.textContent).toContain(
      'read-only shelf cannot build fingerprints'
    );
    expect(
      [...host.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Build fingerprints')
    ).toBe(false);
    expect(mocks.startFingerprintSources).not.toHaveBeenCalled();
  });

  it('shows the empty state when no pair meets the filter', async () => {
    mocks.getSimilarBookPairs.mockResolvedValue([]);
    const host = mount();
    await flush();
    expect(host.querySelector('.similar-empty')).not.toBeNull();
    expect(rows(host)).toHaveLength(0);
  });

  it('shows a narrow-down notice, not an error, when the shelf is too large', async () => {
    mocks.getSimilarBookPairs.mockRejectedValue(new SimilarTooLargeError(5000, 2000));
    const host = mount();
    await flush();

    expect(host.querySelector('.similar-error')).toBeNull();
    const notice = host.querySelector('.similar-notice');
    expect(notice).not.toBeNull();
    expect(notice!.textContent).toContain('5000');
    expect(notice!.textContent).toContain('2000');
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
