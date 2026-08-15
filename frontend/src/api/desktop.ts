import { getActiveShelfID } from './client';
import { isWailsRuntime } from '@/providers/runtime';

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
  ImportBooksFromLocalPaths?: (
    shelfID: string,
    localPaths: string[],
    layerParts: string[]
  ) => Promise<DesktopImportBookResult[]>;
  OpenShelfDirectory?: () => Promise<string>;
  OpenLayerDirectory?: (shelfID: string, layerParts: string[]) => Promise<void>;
  OpenBookDirectory?: (shelfID: string, bookID: string) => Promise<void>;
  AddShelf?: (name: string, libRoot: string, scanInterval: string) => Promise<void>;
  RemoveShelf?: (shelfID: string) => Promise<void>;
  GetShelfDetails?: (shelfID: string) => Promise<DesktopShelfDetails>;
  ModifyShelf?: (shelfID: string, name: string, scanInterval: string) => Promise<void>;
  SaveBookContent?: (shelfID: string, bookID: string, suggestedName: string) => Promise<void>;
  OpenExternalURL?: (url: string) => Promise<void>;
  ReadReadHistory?: () => Promise<string>;
  WriteReadHistory?: (doc: string) => Promise<void>;
  ReadReadingProgress?: () => Promise<string>;
  WriteReadingProgress?: (doc: string) => Promise<void>;
  ReadReadingStats?: () => Promise<string>;
  WriteReadingStats?: (doc: string) => Promise<void>;
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

export async function openDesktopBookFolder(bookID: string): Promise<void> {
  if (!isDesktopRuntime()) {
    return;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenBookDirectory) {
    return;
  }

  await desktopApp.OpenBookDirectory(getActiveShelfID(), bookID);
}

export async function openDesktopLayerFolder(layerPath: string): Promise<void> {
  if (!isDesktopRuntime()) {
    return;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenLayerDirectory) {
    return;
  }

  // normalizeLayerParts splits by '/', trims each segment, and drops empties
  // before passing the layer path into the desktop binding.
  await desktopApp.OpenLayerDirectory(getActiveShelfID(), normalizeLayerParts(layerPath));
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

export async function saveDesktopBookContent(
  shelfID: string,
  bookID: string,
  suggestedName: string
): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.SaveBookContent) {
    throw new Error('SaveBookContent binding not available');
  }

  await desktopApp.SaveBookContent(shelfID, bookID, suggestedName);
}

// Reading history, progress, and stats are stored by the desktop shell itself
// (JSON files next to shelves.json), not by the server and not in WebView
// storage. These bindings are deliberately format-blind: the documents are
// built in storage/readHistory, storage/readingProgress, and
// storage/readingStats; Go only persists the text it is handed.
export function hasDesktopReadHistoryBinding(): boolean {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  return Boolean(desktopApp?.ReadReadHistory && desktopApp?.WriteReadHistory);
}

export async function readDesktopReadHistory(): Promise<string> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ReadReadHistory) {
    throw new Error('ReadReadHistory binding not available');
  }

  return (await desktopApp.ReadReadHistory()) ?? '';
}

export async function writeDesktopReadHistory(doc: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.WriteReadHistory) {
    throw new Error('WriteReadHistory binding not available');
  }

  await desktopApp.WriteReadHistory(doc);
}

export function hasDesktopReadingProgressBinding(): boolean {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  return Boolean(desktopApp?.ReadReadingProgress && desktopApp?.WriteReadingProgress);
}

export async function readDesktopReadingProgress(): Promise<string> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ReadReadingProgress) {
    throw new Error('ReadReadingProgress binding not available');
  }

  return (await desktopApp.ReadReadingProgress()) ?? '';
}

export async function writeDesktopReadingProgress(doc: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.WriteReadingProgress) {
    throw new Error('WriteReadingProgress binding not available');
  }

  await desktopApp.WriteReadingProgress(doc);
}

export function hasDesktopReadingStatsBinding(): boolean {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  return Boolean(desktopApp?.ReadReadingStats && desktopApp?.WriteReadingStats);
}

export async function readDesktopReadingStats(): Promise<string> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ReadReadingStats) {
    throw new Error('ReadReadingStats binding not available');
  }

  return (await desktopApp.ReadReadingStats()) ?? '';
}

export async function writeDesktopReadingStats(doc: string): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.WriteReadingStats) {
    throw new Error('WriteReadingStats binding not available');
  }

  await desktopApp.WriteReadingStats(doc);
}

export async function openDesktopExternalURL(url: string): Promise<void> {
  if (!isDesktopRuntime()) {
    return;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenExternalURL) {
    throw new Error('OpenExternalURL binding not available');
  }

  await desktopApp.OpenExternalURL(url);
}
