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

  // Only the one string turns it on, so a hand-edit or a value an older build
  // wrote cannot accidentally reveal a marked book.
  it.each([['yes'], [''], ['1'], ['TRUE']])('reads the stored %p as off', (stored) => {
    window.localStorage.setItem('show-nsfw-device', stored);

    expect(getShowNsfwOnDevice()).toBe(false);
  });

  it('keeps the shared ref in step with a value another tab wrote', () => {
    const { showNsfw } = useDeviceNsfwPreference();
    window.localStorage.setItem('show-nsfw-device', 'true');

    expect(getShowNsfwOnDevice()).toBe(true);
    expect(showNsfw.value).toBe(true);
  });

  it('exposes one shared ref, so the panel and the lists cannot disagree', () => {
    const panel = useDeviceNsfwPreference();
    const list = useDeviceNsfwPreference();

    panel.setShowNsfw(true);

    expect(list.showNsfw.value).toBe(true);
    expect(panel.showNsfw).toBe(list.showNsfw);
  });
});
