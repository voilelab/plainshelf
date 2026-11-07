import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({
  native: true,
  entries: [] as unknown[],
  listShelves: vi.fn(),
  addShelfEntry: vi.fn(),
  setActiveShelfEntry: vi.fn()
}));

vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => h.native } }));
vi.mock('@/providers/localShelf', () => ({ LocalShelf: { listShelves: h.listShelves } }));
vi.mock('@/providers/mobileConfig', () => ({
  getShelfEntries: () => h.entries,
  addShelfEntry: h.addShelfEntry,
  setActiveShelfEntry: h.setActiveShelfEntry,
  newShelfEntry: () => ({ id: `entry-${h.addShelfEntry.mock.calls.length + 1}`, type: 'local', name: '', localShelfId: '' })
}));

const { adoptLocalShelves } = await import('./adoptLocalShelves');

beforeEach(() => {
  h.native = true;
  h.entries = [];
  h.listShelves.mockReset().mockResolvedValue({ shelves: [] });
  h.addShelfEntry.mockReset().mockResolvedValue(undefined);
  h.setActiveShelfEntry.mockReset().mockResolvedValue(undefined);
});

describe('adoptLocalShelves', () => {
  it('lists shelves that came back without their entries and opens the first', async () => {
    h.listShelves.mockResolvedValue({
      shelves: [
        { id: 'local-1', name: 'Phone' },
        { id: 'local-3', name: 'Notes' }
      ]
    });

    await adoptLocalShelves();

    expect(h.addShelfEntry.mock.calls.map(([entry]) => [entry.name, entry.localShelfId])).toEqual([
      ['Phone', 'local-1'],
      ['Notes', 'local-3']
    ]);
    expect(h.setActiveShelfEntry).toHaveBeenCalledWith('entry-1');
  });

  it('leaves a device with no shelf on disk without a shelf', async () => {
    await adoptLocalShelves();

    expect(h.addShelfEntry).not.toHaveBeenCalled();
    expect(h.setActiveShelfEntry).not.toHaveBeenCalled();
  });

  it('does not look at the disk once the device has entries', async () => {
    h.entries = [{ id: 'e1' }];

    await adoptLocalShelves();

    expect(h.listShelves).not.toHaveBeenCalled();
  });

  it('does not look at the disk off a native device', async () => {
    h.native = false;

    await adoptLocalShelves();

    expect(h.listShelves).not.toHaveBeenCalled();
  });

  it('starts without a shelf when the disk cannot be listed', async () => {
    h.listShelves.mockRejectedValue(new Error('engine not running'));

    await expect(adoptLocalShelves()).resolves.toBeUndefined();
    expect(h.addShelfEntry).not.toHaveBeenCalled();
  });
});
