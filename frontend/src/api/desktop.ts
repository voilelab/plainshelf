import { getActiveShelfID } from './client';
import { isWailsRuntime } from '../providers/runtime';

interface DesktopImportBookResult {
  path?: string;
  id?: string;
  error?: string;
}

export interface DesktopShelfDetails {
  id: string;
  name: string;
  path: string;
  scan_interval: string;
}

interface DesktopAppBinding {
  OpenBookFiles?: () => Promise<string[]>;
  OpenBookFolder?: (shelfID: string, bookID: string) => Promise<void>;
  ImportBooksFromLocalPaths?: (
    shelfID: string,
    localPaths: string[],
    layerParts: string[]
  ) => Promise<DesktopImportBookResult[]>;
  OpenShelfDirectory?: () => Promise<string>;
  AddShelf?: (name: string, libRoot: string, scanInterval: string) => Promise<void>;
  RemoveShelf?: (shelfID: string) => Promise<void>;
  GetShelfDetails?: (shelfID: string) => Promise<DesktopShelfDetails>;
  ModifyShelf?: (shelfID: string, name: string, scanInterval: string) => Promise<void>;
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

export async function openDesktopBookFolder(bookID: string): Promise<void> {
  if (!isDesktopRuntime()) {
    throw new Error('OpenBookFolder is only available in desktop runtime');
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenBookFolder) {
    throw new Error('OpenBookFolder binding not available');
  }

  await desktopApp.OpenBookFolder(getActiveShelfID(), bookID);
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

export async function openDesktopShelfDirectory(): Promise<string | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenShelfDirectory) {
    return null;
  }

  const dir = await desktopApp.OpenShelfDirectory();
  return dir || null;
}

export async function addDesktopShelf(name: string, libRoot: string, scanInterval: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.AddShelf) {
    throw new Error('AddShelf binding not available');
  }

  await desktopApp.AddShelf(name, libRoot, scanInterval);
}

export async function removeDesktopShelf(shelfID: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.RemoveShelf) {
    throw new Error('RemoveShelf binding not available');
  }

  await desktopApp.RemoveShelf(shelfID);
}

export async function getDesktopShelfDetails(shelfID: string): Promise<DesktopShelfDetails> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.GetShelfDetails) {
    throw new Error('GetShelfDetails binding not available');
  }

  return desktopApp.GetShelfDetails(shelfID);
}

export async function modifyDesktopShelf(shelfID: string, name: string, scanInterval: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ModifyShelf) {
    throw new Error('ModifyShelf binding not available');
  }

  await desktopApp.ModifyShelf(shelfID, name, scanInterval);
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
