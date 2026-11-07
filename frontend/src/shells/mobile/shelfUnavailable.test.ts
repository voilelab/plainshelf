import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({
  entry: null as { id: string } | null,
  usable: false
}));

vi.mock('@/providers/mobileConfig', () => ({
  getActiveShelfEntry: () => h.entry,
  isActiveShelfEntryUsable: () => h.usable
}));

const { mobileShelfUnavailable } = await import('./shelfUnavailable');

beforeEach(() => {
  h.entry = null;
  h.usable = false;
});

describe('mobileShelfUnavailable', () => {
  it('sends a device with no shelf to the shelf settings', () => {
    expect(mobileShelfUnavailable()).toEqual({
      messageKey: 'layout.shelf.unavailableDescription',
      actionKey: 'layout.shelf.manage',
      fixTo: { path: '/settings', query: { tab: 'shelves' } }
    });
  });

  it('sends a shelf missing its token to its own editor', () => {
    h.entry = { id: 'e1' };

    expect(mobileShelfUnavailable()).toEqual({
      messageKey: 'layout.shelf.unusableDescription',
      actionKey: 'layout.shelf.fix',
      fixTo: { name: 'mobile-shelf-edit', params: { entryId: 'e1' } }
    });
  });

  it('reports nothing for a usable shelf', () => {
    h.entry = { id: 'e1' };
    h.usable = true;

    expect(mobileShelfUnavailable()).toBeNull();
  });
});
