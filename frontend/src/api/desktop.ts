import { getActiveShelfID } from './client';
import { isWailsRuntime } from '../providers/runtime';

interface DesktopImportBookResult {
  path?: string;
  id?: string;
  error?: string;
}

interface DesktopAppBinding {
  OpenBookFiles?: () => Promise<string[]>;
  ImportBooksFromLocalPaths?: (
    shelfID: string,
    localPaths: string[],
    layerParts: string[]
  ) => Promise<DesktopImportBookResult[]>;
}

interface DesktopWindow extends Window {
  go?: {
    main?: {
      DesktopApp?: DesktopAppBinding;
    };
  };
}

export function isDesktopRuntime(): boolean {
  return isWailsRuntime();
}

export async function openDesktopBookFiles(): Promise<string[] | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenBookFiles) {
    return null;
  }

  return desktopApp.OpenBookFiles();
}

function normalizeLayerParts(layerPath: string): string[] {
  const trimmed = layerPath.trim();
  if (!trimmed || trimmed === '/') {
    return [];
  }

  return trimmed
    .split('/')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
}

export async function importDesktopBooksFromLocalPaths(
  localPaths: string[],
  layerPath: string
): Promise<DesktopImportBookResult[] | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ImportBooksFromLocalPaths) {
    return null;
  }

  return desktopApp.ImportBooksFromLocalPaths(
    getActiveShelfID(),
    localPaths,
    normalizeLayerParts(layerPath)
  );
}

const desktopZoomStep = 0.1;
const desktopMinZoom = 0.5;
const desktopMaxZoom = 2;

type DesktopZoomAction = 'in' | 'out' | 'reset';

function readDesktopZoom(): number {
  const currentZoom = Number(document.documentElement.dataset.plainshelfZoom ?? '1');
  if (Number.isFinite(currentZoom)) {
    return currentZoom;
  }

  return 1;
}

function applyDesktopZoom(action: DesktopZoomAction): void {
  const currentZoom = readDesktopZoom();
  const nextZoom = action === 'reset'
    ? 1
    : Math.min(
      desktopMaxZoom,
      Math.max(desktopMinZoom, currentZoom + (action === 'in' ? desktopZoomStep : -desktopZoomStep))
    );
  const roundedZoom = Math.round(nextZoom * 100) / 100;

  document.documentElement.dataset.plainshelfZoom = String(roundedZoom);
  if (roundedZoom === 1) {
    document.documentElement.style.removeProperty('zoom');
  } else {
    document.documentElement.style.setProperty('zoom', String(roundedZoom));
  }
}

function desktopZoomActionForKey(event: KeyboardEvent): DesktopZoomAction | null {
  if (event.defaultPrevented || !(event.ctrlKey || event.metaKey) || event.altKey) {
    return null;
  }

  switch (event.key) {
    case '+':
    case '=':
      return 'in';
    case '-':
    case '_':
      return 'out';
    case '0':
      return 'reset';
    default:
      return null;
  }
}

export function installDesktopZoomShortcuts(): void {
  if (!isDesktopRuntime()) {
    return;
  }

  window.addEventListener('keydown', (event) => {
    const action = desktopZoomActionForKey(event);
    if (!action) {
      return;
    }

    event.preventDefault();
    applyDesktopZoom(action);
  });
}
