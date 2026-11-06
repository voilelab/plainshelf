import { afterEach, describe, expect, it, vi } from 'vitest';

const stored = new Map<string, string>([['plainshelf.shelf', 'main']]);

vi.mock('@/storage/preferences', () => ({
  getPreference: (key: string) => stored.get(key) ?? null,
  setPreference: (key: string, value: string) => void stored.set(key, value)
}));

function stubWindow(search: string, desktop: boolean): void {
  vi.stubGlobal('window', { location: { search }, __PLAINSHELF_DESKTOP__: desktop });
}

async function freshClient(): Promise<typeof import('./client')> {
  vi.resetModules();
  return import('./client');
}

describe('active shelf of a desktop reader window', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('comes from the window URL without changing the saved shelf', async () => {
    stubWindow('?shelf=second&section=2', true);

    const { getActiveShelfID } = await freshClient();

    expect(getActiveShelfID()).toBe('second');
    expect(stored.get('plainshelf.shelf')).toBe('main');
  });

  it('falls back to the saved shelf without a shelf parameter', async () => {
    stubWindow('', true);

    const { getActiveShelfID } = await freshClient();

    expect(getActiveShelfID()).toBe('main');
  });

  it('ignores the URL outside the desktop app', async () => {
    stubWindow('?shelf=second', false);

    const { getActiveShelfID } = await freshClient();

    expect(getActiveShelfID()).toBe('main');
  });
});
