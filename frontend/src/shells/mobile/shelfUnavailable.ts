import type { ShelfUnavailable } from '@/providers/shell';
import { getActiveShelfEntry, isActiveShelfEntryUsable } from '@/providers/mobileConfig';

export const MANAGE_SHELVES_ROUTE = { path: '/settings', query: { tab: 'shelves' } };

export function mobileShelfUnavailable(): ShelfUnavailable | null {
  const entry = getActiveShelfEntry();
  if (!entry) {
    return {
      messageKey: 'layout.shelf.unavailableDescription',
      actionKey: 'layout.shelf.manage',
      fixTo: MANAGE_SHELVES_ROUTE
    };
  }
  if (isActiveShelfEntryUsable()) {
    return null;
  }
  return {
    messageKey: 'layout.shelf.unusableDescription',
    actionKey: 'layout.shelf.fix',
    fixTo: { name: 'mobile-shelf-edit', params: { entryId: entry.id } }
  };
}
