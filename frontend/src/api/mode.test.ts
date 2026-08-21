import { beforeEach, describe, expect, it, vi } from 'vitest';

const { fetchJsonMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn()
}));

vi.mock('./client', () => ({
  fetchJson: fetchJsonMock,
  isMockApiMode: () => false
}));

const { getReadOnlyMode, getServerMode, resetServerModeCache } = await import('./mode');

beforeEach(() => {
  fetchJsonMock.mockReset();
  resetServerModeCache();
});

describe('getServerMode', () => {
  it('reads the mode the server names', async () => {
    fetchJsonMock.mockResolvedValue({ read_only: true, mode: 'reader' });

    await expect(getServerMode()).resolves.toEqual({ readOnly: true, mode: 'reader' });
  });

  // A build that meets a mode it does not know about should offer the surface it
  // has always offered and let the routes answer for what is missing, rather
  // than gate pages on a name it cannot interpret.
  it.each([
    ['an unknown name', { mode: 'kiosk' }],
    ['no mode at all', {}],
    ['a non-string mode', { mode: 3 }]
  ])('falls back to full for %s', async (_label, response) => {
    fetchJsonMock.mockResolvedValue(response);

    await expect(getServerMode()).resolves.toMatchObject({ mode: 'full' });
  });

  // The server's mode is fixed for the life of the process behind it, and both
  // the reader shell and MainLayout ask during startup.
  it('issues one request for every caller', async () => {
    fetchJsonMock.mockResolvedValue({ read_only: false, mode: 'full' });

    await Promise.all([getServerMode(), getReadOnlyMode()]);
    await getServerMode();

    expect(fetchJsonMock).toHaveBeenCalledTimes(1);
  });

  // Caching a rejection would leave a client that started before its server did
  // stuck reporting a failure it could otherwise recover from.
  it('does not remember a failure', async () => {
    fetchJsonMock.mockRejectedValueOnce(new Error('offline'));
    await expect(getServerMode()).rejects.toThrow('offline');

    fetchJsonMock.mockResolvedValue({ read_only: true, mode: 'reader' });
    await expect(getServerMode()).resolves.toEqual({ readOnly: true, mode: 'reader' });
    expect(fetchJsonMock).toHaveBeenCalledTimes(2);
  });
});

describe('getReadOnlyMode', () => {
  it.each([
    [true, true],
    ['true', true],
    [1, true],
    ['1', true],
    [false, false],
    [undefined, false]
  ])('reads read_only %p as %p', async (value, expected) => {
    fetchJsonMock.mockResolvedValue({ read_only: value });

    await expect(getReadOnlyMode()).resolves.toBe(expected);
  });
});
