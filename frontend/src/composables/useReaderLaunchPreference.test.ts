/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  getReaderLaunchMode,
  parseReaderLaunchMode,
  readerLaunchModes,
  setReaderLaunchMode
} from './useReaderLaunchPreference';

type DesktopWindow = Window & { __PLAINSHELF_DESKTOP__?: boolean };

describe('parseReaderLaunchMode', () => {
  it('defaults to opening a new reader when never set', () => {
    expect(parseReaderLaunchMode(null)).toBe('new-reader');
  });

  it('does not offer a new window outside the desktop app', () => {
    expect(readerLaunchModes()).not.toContain('new-window');
    expect(parseReaderLaunchMode('new-window')).toBe('new-reader');
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

  it('reflects a value written directly to storage by another tab', () => {
    window.localStorage.setItem('reader-launch-mode', 'in-window');

    expect(getReaderLaunchMode()).toBe('in-window');
  });
});

describe('reader launch mode in the desktop app', () => {
  beforeEach(() => {
    (window as DesktopWindow).__PLAINSHELF_DESKTOP__ = true;
  });

  afterEach(() => {
    delete (window as DesktopWindow).__PLAINSHELF_DESKTOP__;
  });

  it('defaults to a new window when never set', () => {
    expect(parseReaderLaunchMode(null)).toBe('new-window');
  });

  it('keeps an explicit earlier choice', () => {
    expect(parseReaderLaunchMode('new-reader')).toBe('new-reader');
    expect(parseReaderLaunchMode('in-window')).toBe('in-window');
  });

  it('lists the new window first', () => {
    expect(readerLaunchModes()).toEqual(['new-window', 'new-reader', 'in-window']);
  });
});
