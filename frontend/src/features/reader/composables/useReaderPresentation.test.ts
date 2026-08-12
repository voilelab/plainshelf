import { describe, expect, it } from 'vitest';
import { shouldUseMobileReader } from './useReaderPresentation';

describe('reader presentation selection', () => {
  it('always uses the mobile reader in the mobile runtime', () => {
    expect(shouldUseMobileReader(true, false)).toBe(true);
    expect(shouldUseMobileReader(true, true)).toBe(true);
  });

  it('uses the mobile reader for narrow server pages only', () => {
    expect(shouldUseMobileReader(false, true)).toBe(true);
    expect(shouldUseMobileReader(false, false)).toBe(false);
  });
});
