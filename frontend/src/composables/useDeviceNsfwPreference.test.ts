/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it } from 'vitest';
import {
  getShowNsfwOnDevice,
  setShowNsfwOnDevice,
  useDeviceNsfwPreference
} from './useDeviceNsfwPreference';

describe('useDeviceNsfwPreference', () => {
  beforeEach(() => {
    window.localStorage.clear();
    setShowNsfwOnDevice(false);
    window.localStorage.clear();
  });

  // Hiding is the safe direction to fail in: a device that cannot tell shows
  // nothing rather than everything.
  it('defaults to hiding adult content when nothing is stored', () => {
    expect(getShowNsfwOnDevice()).toBe(false);
    expect(useDeviceNsfwPreference().showNsfw.value).toBe(false);
  });

  it('persists the choice to localStorage and reads it back', () => {
    setShowNsfwOnDevice(true);

    expect(getShowNsfwOnDevice()).toBe(true);
    expect(window.localStorage.getItem('show-nsfw-device')).toBe('true');

    setShowNsfwOnDevice(false);
    expect(getShowNsfwOnDevice()).toBe(false);
    expect(window.localStorage.getItem('show-nsfw-device')).toBe('false');
  });
});
