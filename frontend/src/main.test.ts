import { expect, it, vi } from 'vitest';

const calls = vi.hoisted((): string[] => []);

vi.mock('@/api/wailsBindings', () => ({ startWailsRuntime: () => calls.push('runtime') }));
vi.mock('@/storage/preferences', () => ({
  loadPreferences: async () => {
    calls.push('preferences');
  }
}));
vi.mock('./bootstrap', () => ({
  bootstrap: async () => {
    calls.push('bootstrap');
  }
}));

it('starts the Wails runtime before anything else boots', async () => {
  await import('./main');

  await vi.waitFor(() => expect(calls).toEqual(['runtime', 'preferences', 'bootstrap']));
});
