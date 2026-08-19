import { describe, expect, it, vi } from 'vitest';

const { fetchJsonMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn()
}));

vi.mock('./client', async () => {
  const actual = await vi.importActual<typeof import('./client')>('./client');
  return {
    ApiError: actual.ApiError,
    fetchJson: fetchJsonMock,
    buildShelfApiPath: (path: string) => path,
    isMockApiMode: () => false
  };
});

const { ApiError } = await import('./client');
const { createLayer } = await import('./layers');

describe('createLayer', () => {
  // The server refuses a name the shelf scanner would skip with an explanation
  // the user needs; replacing every 400 with one fixed sentence hid it.
  it('surfaces the reason the server gave for a rejected name', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError('invalid layer name: hidden and system directory names are skipped', { status: 400 })
    );

    await expect(createLayer('@eaDir')).rejects.toThrow(
      'invalid layer name: hidden and system directory names are skipped'
    );
  });

  it('still reports a generic failure for a server error', async () => {
    fetchJsonMock.mockRejectedValueOnce(new ApiError('boom', { status: 500 }));

    await expect(createLayer('Fiction')).rejects.toThrow('Failed to create layer');
  });
});
