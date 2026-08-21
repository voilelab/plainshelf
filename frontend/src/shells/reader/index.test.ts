import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { getServerModeMock } = vi.hoisted(() => ({
  getServerModeMock: vi.fn()
}));

vi.mock('@/api/mode', () => ({
  getServerMode: getServerModeMock
}));

const { installReaderShell } = await import('./index');
const { getShell, registerShell } = await import('@/providers/shell');
const { isWritableProvider } = await import('@/providers');

beforeEach(() => {
  getServerModeMock.mockReset();
});

afterEach(() => {
  registerShell(null);
});

describe('installReaderShell', () => {
  it('installs the shell when the server reports reader mode', async () => {
    getServerModeMock.mockResolvedValue({ readOnly: true, mode: 'reader' });

    await expect(installReaderShell()).resolves.toBe(true);

    const shell = getShell();
    expect(shell?.installRouterGuards).toBeTypeOf('function');
    expect(isWritableProvider(shell!.createProvider())).toBe(false);
  });

  it('leaves an ordinary server as a plain web client', async () => {
    getServerModeMock.mockResolvedValue({ readOnly: false, mode: 'full' });

    await expect(installReaderShell()).resolves.toBe(false);
    expect(getShell()).toBeNull();
  });

  // A read-only server is still a full one: it serves every route and
  // administers itself, so its editing UI stays where a read-only banner and
  // refused writes can explain themselves.
  it('does not treat a read-only server as a reader', async () => {
    getServerModeMock.mockResolvedValue({ readOnly: true, mode: 'full' });

    await expect(installReaderShell()).resolves.toBe(false);
    expect(getShell()).toBeNull();
  });

  // Failing open is the safe direction: a shell that is not installed costs a
  // reader some blocked routes, while one installed by mistake would take the
  // editing UI away from a full server.
  it('installs nothing when the probe fails', async () => {
    getServerModeMock.mockRejectedValue(new Error('offline'));

    await expect(installReaderShell()).resolves.toBe(false);
    expect(getShell()).toBeNull();
  });
});
