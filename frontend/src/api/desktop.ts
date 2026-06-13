import { getActiveShelfID } from './client';
import { isWailsRuntime } from '../providers/runtime';

interface DesktopImportBookResult {
  path?: string;
  id?: string;
  error?: string;
}

export interface DesktopShelfInfo {
  id: string;
  name: string;
  lib_root: string;
}

interface DesktopAppBinding {
  OpenShelfDirectory?: () => Promise<string>;
  AddShelf?: (name: string, libRoot: string) => Promise<DesktopShelfInfo>;
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

export async function openDesktopShelfDirectory(): Promise<string | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenShelfDirectory) {
    return null;
  }

  const selectedPath = await desktopApp.OpenShelfDirectory();
  return selectedPath.trim() || null;
}

export async function addDesktopShelf(name: string, libRoot: string): Promise<DesktopShelfInfo | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.AddShelf) {
    return null;
  }

  return desktopApp.AddShelf(name, libRoot);
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
