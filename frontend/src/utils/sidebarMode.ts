export type SidebarMode = 'expanded' | 'rail';

export const SIDEBAR_MODE_STORAGE_KEY = 'plainshelf.sidebar.mode';
export const SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY = 'plainshelf.sidebar.expandedWidth';

export const MIN_EXPANDED_SIDEBAR_WIDTH = 200;
export const MAX_EXPANDED_SIDEBAR_WIDTH = 300;

const SIDEBAR_MODES: SidebarMode[] = ['expanded', 'rail'];

export function isSidebarMode(value: string | null | undefined): value is SidebarMode {
  return value !== null && value !== undefined && SIDEBAR_MODES.includes(value as SidebarMode);
}

export function getStoredSidebarMode(): SidebarMode {
  if (typeof window === 'undefined') {
    return 'expanded';
  }

  const value = window.localStorage.getItem(SIDEBAR_MODE_STORAGE_KEY);
  return isSidebarMode(value) ? value : 'expanded';
}

export function setStoredSidebarMode(mode: SidebarMode): void {
  if (typeof window === 'undefined') {
    return;
  }

  window.localStorage.setItem(SIDEBAR_MODE_STORAGE_KEY, mode);
}

function isExpandedSidebarWidth(value: number): boolean {
  return Number.isFinite(value) && value >= MIN_EXPANDED_SIDEBAR_WIDTH && value <= MAX_EXPANDED_SIDEBAR_WIDTH;
}

export function getStoredSidebarExpandedWidth(): number | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const raw = window.localStorage.getItem(SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY);
  if (raw === null) {
    return null;
  }

  const value = Number.parseFloat(raw);
  return isExpandedSidebarWidth(value) ? value : null;
}

export function setStoredSidebarExpandedWidth(width: number): void {
  if (typeof window === 'undefined' || !isExpandedSidebarWidth(width)) {
    return;
  }

  window.localStorage.setItem(SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY, String(width));
}
