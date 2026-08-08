import { beforeEach, describe, expect, it, vi } from 'vitest';

const { prefGet, prefSet, prefRemove } = vi.hoisted(() => ({
  prefGet: vi.fn(),
  prefSet: vi.fn(),
  prefRemove: vi.fn()
}));

vi.mock('@capacitor/preferences', () => ({
  Preferences: { get: prefGet, set: prefSet, remove: prefRemove }
}));

const { SecureStorage } = await import('./secureStorage');

describe('SecureStorage web implementation', () => {
  beforeEach(() => {
    prefGet.mockReset();
    prefSet.mockReset();
    prefRemove.mockReset();
  });

  it('persists browser-preview values in a separate Preferences namespace', async () => {
    prefGet.mockResolvedValue({ value: 'token' });

    await SecureStorage.set({ key: 'accessToken', value: 'token' });
    expect(await SecureStorage.get({ key: 'accessToken' })).toEqual({ value: 'token' });
    await SecureStorage.remove({ key: 'accessToken' });

    expect(prefSet).toHaveBeenCalledWith({
      key: 'plainshelf.secure.accessToken',
      value: 'token'
    });
    expect(prefGet).toHaveBeenCalledWith({ key: 'plainshelf.secure.accessToken' });
    expect(prefRemove).toHaveBeenCalledWith({ key: 'plainshelf.secure.accessToken' });
  });
});
