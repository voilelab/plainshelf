import { beforeEach, expect, it } from 'vitest';
import { useShelvesStore } from './useShelvesStore';
import { useShelfCapabilities } from './useShelfCapabilities';
const store = useShelvesStore();
beforeEach(() => { store.shelves.value = []; store.selectedShelfID.value = ''; });
it('keeps older server behavior, without inventing an offline download layer', () => {
  const caps = useShelfCapabilities();
  expect([caps.rescan.value, caps.charCount.value, caps.fingerprint.value, caps.download.value, caps.snapshotAt.value])
    .toEqual([true, true, true, false, null]);
});
it('follows engine answers and shelf selection reactively', () => {
  store.shelves.value = [
    { id: 'cloud', name: 'Cloud', capabilities: { rescan: false, char_count: false, fingerprint: false, download: true }, offline: { snapshot_at: '2026-10-16T09:12:44Z' } },
    { id: 'local', name: 'Local', capabilities: { download: false } }
  ];
  store.selectShelf('cloud');
  const caps = useShelfCapabilities();
  expect([caps.rescan.value, caps.charCount.value, caps.fingerprint.value, caps.download.value])
    .toEqual([false, false, false, true]);
  expect(caps.snapshotAt.value).toBe(Date.parse('2026-10-16T09:12:44Z'));
  store.selectShelf('local');
  expect([caps.rescan.value, caps.charCount.value, caps.fingerprint.value, caps.download.value, caps.snapshotAt.value])
    .toEqual([true, true, true, false, null]);
});
