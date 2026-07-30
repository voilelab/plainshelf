export type SidebarMode = 'expanded' | 'rail';

export const SIDEBAR_MODE_STORAGE_KEY = 'plainshelf.sidebar.mode';

const SIDEBAR_MODES: SidebarMode[] = ['expanded', 'rail'];

export function isSidebarMode(value: string | null | undefined): value is SidebarMode {
  return value !== null && value !== undefined && SIDEBAR_MODES.includes(value as SidebarMode);
}

function resolveStorageKey(key?: string): string {
  return key && key.trim().length > 0 ? key : SIDEBAR_MODE_STORAGE_KEY;
}

export function getStoredSidebarMode(storageKey?: string): SidebarMode {
  if (typeof window === 'undefined') {
    return 'expanded';
  }

  const value = window.localStorage.getItem(resolveStorageKey(storageKey));
  return isSidebarMode(value) ? value : 'expanded';
}

export function setStoredSidebarMode(mode: SidebarMode, storageKey?: string): void {
  if (typeof window === 'undefined') {
    return;
  }

  window.localStorage.setItem(resolveStorageKey(storageKey), mode);
}
