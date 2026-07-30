export type SidebarMode = 'expanded' | 'rail';

export const SIDEBAR_MODE_STORAGE_KEY = 'plainshelf.sidebar.mode';
export const SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY = 'plainshelf.sidebar.expandedWidth';

export const MIN_EXPANDED_SIDEBAR_WIDTH = 200;
export const MAX_EXPANDED_SIDEBAR_WIDTH = 300;

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

function resolveWidthStorageKey(key?: string): string {
  return key && key.trim().length > 0 ? key : SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY;
}

function isExpandedSidebarWidth(value: number): boolean {
  return Number.isFinite(value) && value >= MIN_EXPANDED_SIDEBAR_WIDTH && value <= MAX_EXPANDED_SIDEBAR_WIDTH;
}

export function getStoredSidebarExpandedWidth(storageKey?: string): number | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const raw = window.localStorage.getItem(resolveWidthStorageKey(storageKey));
  if (raw === null) {
    return null;
  }

  const value = Number.parseFloat(raw);
  return isExpandedSidebarWidth(value) ? value : null;
}

export function setStoredSidebarExpandedWidth(width: number, storageKey?: string): void {
  if (typeof window === 'undefined' || !isExpandedSidebarWidth(width)) {
    return;
  }

  window.localStorage.setItem(resolveWidthStorageKey(storageKey), String(width));
}
