import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY,
  SIDEBAR_MODE_STORAGE_KEY,
  getStoredSidebarExpandedWidth,
  getStoredSidebarMode,
  isSidebarMode,
  setStoredSidebarExpandedWidth,
  setStoredSidebarMode
} from './sidebarMode';

// Tests run in a node environment; the util only needs localStorage on window.
function stubWindow(store: Map<string, string>): void {
  vi.stubGlobal('window', {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      }
    }
  });
}

describe('sidebarMode', () => {
  let store: Map<string, string>;

  beforeEach(() => {
    store = new Map();
    stubWindow(store);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('uses the plainshelf-prefixed storage key', () => {
    expect(SIDEBAR_MODE_STORAGE_KEY).toBe('plainshelf.sidebar.mode');
  });

  it('recognizes only the two supported modes', () => {
    expect(isSidebarMode('expanded')).toBe(true);
    expect(isSidebarMode('rail')).toBe(true);
    expect(isSidebarMode('collapsed')).toBe(false);
    expect(isSidebarMode(null)).toBe(false);
    expect(isSidebarMode(undefined)).toBe(false);
  });

  it('defaults to expanded when nothing is stored', () => {
    expect(getStoredSidebarMode()).toBe('expanded');
  });

  it('defaults to expanded when the stored value is invalid', () => {
    store.set(SIDEBAR_MODE_STORAGE_KEY, 'sideways');
    expect(getStoredSidebarMode()).toBe('expanded');
  });

  it('round-trips rail and expanded through storage', () => {
    setStoredSidebarMode('rail');
    expect(store.get(SIDEBAR_MODE_STORAGE_KEY)).toBe('rail');
    expect(getStoredSidebarMode()).toBe('rail');

    setStoredSidebarMode('expanded');
    expect(getStoredSidebarMode()).toBe('expanded');
  });

  it('falls back to expanded without a window', () => {
    vi.unstubAllGlobals();
    expect(getStoredSidebarMode()).toBe('expanded');
    expect(() => setStoredSidebarMode('rail')).not.toThrow();
  });

  describe('expanded width', () => {
    it('round-trips a width inside the resizable range', () => {
      setStoredSidebarExpandedWidth(276);
      expect(store.get(SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY)).toBe('276');
      expect(getStoredSidebarExpandedWidth()).toBe(276);
    });

    it('returns null when nothing is stored', () => {
      expect(getStoredSidebarExpandedWidth()).toBeNull();
    });

    it('ignores widths outside the resizable range', () => {
      setStoredSidebarExpandedWidth(48);
      setStoredSidebarExpandedWidth(1000);
      expect(getStoredSidebarExpandedWidth()).toBeNull();
    });

    it('ignores an unparsable stored width', () => {
      store.set(SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY, 'wide');
      expect(getStoredSidebarExpandedWidth()).toBeNull();
    });

    it('is inert without a window', () => {
      vi.unstubAllGlobals();
      expect(getStoredSidebarExpandedWidth()).toBeNull();
      expect(() => setStoredSidebarExpandedWidth(260)).not.toThrow();
    });
  });
});
