import { beforeEach, describe, expect, it, vi } from 'vitest';

const { isMobileRuntimeMock } = vi.hoisted(() => ({
  isMobileRuntimeMock: vi.fn()
}));

vi.mock('@/providers/runtime', () => ({
  isMobileRuntime: isMobileRuntimeMock
}));

const { useServerMode } = await import('./useServerMode');
const { isLibraryEditingSupported, useWriteAccess } = await import('./useWriteAccess');

// readOnly is a module-level ref shared by every useServerMode() caller, so the
// test drives it directly instead of stubbing GET /api/mode.
const { readOnly } = useServerMode();

describe('useWriteAccess', () => {
  beforeEach(() => {
    isMobileRuntimeMock.mockReset();
    readOnly.value = false;
  });

  it('enables writes on a writable server outside the mobile shell', () => {
    isMobileRuntimeMock.mockReturnValue(false);
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(true);
    expect(writeDisabledReason.value).toBeNull();
  });

  it('disables writes in the mobile shell even when the server is writable', () => {
    isMobileRuntimeMock.mockReturnValue(true);
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('platform');
  });

  it('disables writes when the server is read-only outside the mobile shell', () => {
    isMobileRuntimeMock.mockReturnValue(false);
    readOnly.value = true;
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('server-read-only');
  });

  it('reports the platform reason first when both apply', () => {
    isMobileRuntimeMock.mockReturnValue(true);
    readOnly.value = true;
    const { writesEnabled, writeDisabledReason } = useWriteAccess();

    expect(writesEnabled.value).toBe(false);
    expect(writeDisabledReason.value).toBe('platform');
  });

  it('tracks server mode changes without re-creating the composable', () => {
    isMobileRuntimeMock.mockReturnValue(false);
    const { writesEnabled } = useWriteAccess();
    expect(writesEnabled.value).toBe(true);

    readOnly.value = true;
    expect(writesEnabled.value).toBe(false);
  });
});

describe('isLibraryEditingSupported', () => {
  it('mirrors the mobile runtime check', () => {
    isMobileRuntimeMock.mockReturnValue(true);
    expect(isLibraryEditingSupported()).toBe(false);

    isMobileRuntimeMock.mockReturnValue(false);
    expect(isLibraryEditingSupported()).toBe(true);
  });
});
