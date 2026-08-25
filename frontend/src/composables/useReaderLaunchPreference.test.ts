/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it } from 'vitest';
import {
  DEFAULT_READER_LAUNCH_MODE,
  getReaderLaunchMode,
  parseReaderLaunchMode,
  setReaderLaunchMode
} from './useReaderLaunchPreference';

describe('parseReaderLaunchMode', () => {
  it('defaults to opening a new reader when never set', () => {
    expect(parseReaderLaunchMode(null)).toBe('new-reader');
    expect(DEFAULT_READER_LAUNCH_MODE).toBe('new-reader');
  });

  it('falls back to the default on an unknown stored value', () => {
    expect(parseReaderLaunchMode('sideways')).toBe('new-reader');
    expect(parseReaderLaunchMode('')).toBe('new-reader');
  });

  it('keeps a recognised mode', () => {
    expect(parseReaderLaunchMode('new-reader')).toBe('new-reader');
    expect(parseReaderLaunchMode('in-window')).toBe('in-window');
  });
});

describe('setReaderLaunchMode / getReaderLaunchMode', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('persists the chosen mode to localStorage and reads it back', () => {
    setReaderLaunchMode('in-window');

    expect(getReaderLaunchMode()).toBe('in-window');
    expect(window.localStorage.getItem('reader-launch-mode')).toBe('in-window');
  });

  it('coerces an unexpected value back to the default before storing', () => {
    setReaderLaunchMode('nonsense' as 'in-window');

    expect(getReaderLaunchMode()).toBe('new-reader');
    expect(window.localStorage.getItem('reader-launch-mode')).toBe('new-reader');
  });

  // The getter reads straight from storage, so a value written by another tab
  // (here, a direct localStorage write) is picked up rather than a stale cached
  // ref — the cross-tab case the Codex review flagged.
  it('reflects a value written directly to storage by another tab', () => {
    window.localStorage.setItem('reader-launch-mode', 'in-window');

    expect(getReaderLaunchMode()).toBe('in-window');
  });
});
