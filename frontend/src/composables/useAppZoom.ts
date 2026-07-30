import { ref, watch } from 'vue';
import { isWailsRuntime } from '@/providers/runtime';

const STORAGE_KEY = 'app-zoom-level';
const DEFAULT_ZOOM = 1;
const ZOOM_LEVELS = [0.5, 0.67, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2];

declare global {
  interface Window {
    __plainshelfZoom?: (action: 'in' | 'out' | 'reset') => void;
  }
}

function closestZoomLevel(value: number): number {
  let closest = ZOOM_LEVELS[0];
  let smallestDiff = Math.abs(ZOOM_LEVELS[0] - value);

  for (const level of ZOOM_LEVELS) {
    const diff = Math.abs(level - value);
    if (diff < smallestDiff) {
      smallestDiff = diff;
      closest = level;
    }
  }

  return closest;
}

function clampZoomLevel(value: number): number {
  if (ZOOM_LEVELS.includes(value)) {
    return value;
  }
  return closestZoomLevel(value);
}

function parseZoomLevel(rawValue: string | null): number {
  if (!rawValue) {
    return DEFAULT_ZOOM;
  }

  const parsed = Number.parseFloat(rawValue);
  if (Number.isNaN(parsed) || !Number.isFinite(parsed)) {
    return DEFAULT_ZOOM;
  }

  return clampZoomLevel(parsed);
}

// Module-level state, mirroring useReaderSettings.ts: a single shared zoom
// level for the whole app rather than per-component state, since zoom is a
// desktop-shell-wide concern driven by the native menu, not a Vue component.
const appZoom = ref(DEFAULT_ZOOM);

if (typeof window !== 'undefined') {
  appZoom.value = parseZoomLevel(window.localStorage.getItem(STORAGE_KEY));
}

function applyZoom(zoom: number): void {
  if (typeof document === 'undefined') {
    return;
  }

  const el = document.documentElement;
  el.style.setProperty('zoom', String(zoom));
  el.style.setProperty('--app-zoom', String(zoom));
}

watch(
  appZoom,
  (value) => {
    applyZoom(value);

    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, String(clampZoomLevel(value)));
  },
  { immediate: false }
);

function stepZoom(direction: 1 | -1): void {
  const currentIndex = ZOOM_LEVELS.indexOf(clampZoomLevel(appZoom.value));
  const nextIndex = Math.min(ZOOM_LEVELS.length - 1, Math.max(0, currentIndex + direction));
  appZoom.value = ZOOM_LEVELS[nextIndex];
}

function zoomIn(): void {
  stepZoom(1);
}

function zoomOut(): void {
  stepZoom(-1);
}

function resetZoom(): void {
  appZoom.value = DEFAULT_ZOOM;
}

export function initAppZoom(): void {
  if (!isWailsRuntime()) {
    return;
  }

  // Apply the stored zoom immediately; the watcher above only fires on
  // subsequent changes, not for the initial value read above.
  applyZoom(appZoom.value);

  window.__plainshelfZoom = (action: 'in' | 'out' | 'reset') => {
    if (action === 'in') {
      zoomIn();
    } else if (action === 'out') {
      zoomOut();
    } else if (action === 'reset') {
      resetZoom();
    }
  };
}
