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
  OpenShelfDirectory?: () => Promise<string>;
  AddShelf?: (name: string, libRoot: string) => Promise<void>;
  RemoveShelf?: (shelfID: string) => Promise<void>;
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

export async function addDesktopShelf(name: string, libRoot: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.AddShelf) {
    throw new Error('AddShelf binding not available');
  }

  await desktopApp.AddShelf(name, libRoot);
}

export async function removeDesktopShelf(shelfID: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.RemoveShelf) {
    throw new Error('RemoveShelf binding not available');
  }

  await desktopApp.RemoveShelf(shelfID);
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
