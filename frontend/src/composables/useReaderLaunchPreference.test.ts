/**
 * @vitest-environment jsdom
 */
import { nextTick } from 'vue';
import { describe, expect, it } from 'vitest';
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

// The module is a shared singleton whose watcher persists only on an actual
// change, so each case transitions from a distinct baseline rather than sharing
// a reset — setting the same value twice would not write anything.
describe('setReaderLaunchMode', () => {
  it('persists the chosen mode to localStorage and reads it back', async () => {
    setReaderLaunchMode('new-reader');
    await nextTick();

    setReaderLaunchMode('in-window');

    // The ref updates synchronously; localStorage is written by the watcher.
    expect(getReaderLaunchMode()).toBe('in-window');
    await nextTick();
    expect(window.localStorage.getItem('reader-launch-mode')).toBe('in-window');
  });

  it('coerces an unexpected value back to the default before storing', async () => {
    setReaderLaunchMode('in-window');
    await nextTick();

    setReaderLaunchMode('nonsense' as 'in-window');

    expect(getReaderLaunchMode()).toBe('new-reader');
    await nextTick();
    expect(window.localStorage.getItem('reader-launch-mode')).toBe('new-reader');
  });
});
