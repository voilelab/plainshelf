import { getActiveShelfID } from './client';
import { isWailsRuntime } from '@/providers/runtime';
import type { ShelfSourceCheck, ShelfSourceKind } from '@/utils/shelfSourceCheck';

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
  book_check_interval: string;
  read_only: boolean;
  source: ShelfSourceKind | '';
  url: string;
  username: string;
  open_error?: string;
}

export interface DesktopShelfSourceParams {
  source: ShelfSourceKind | '';
  url: string;
  username: string;
  secret: string;
}

export interface DesktopAddShelfParams {
  name: string;
  libRoot: string;
  scanInterval: string;
  bookCheckInterval: string;
  readOnly: boolean;
  remote?: DesktopShelfSourceParams;
}

export interface DesktopShelfIDPreview {
  id: string;
  defaultPath: string;
}

export interface DesktopModifyShelfParams {
  shelfID: string;
  name: string;
  scanInterval: string;
  bookCheckInterval: string;
  readOnly: boolean;
}

export interface DesktopLANShareStatus {
  enabled: boolean;
  running: boolean;
  host: string;
  addresses: string[];
  shelves: { id: string; name: string; shared: boolean }[];
  url: string;
  token: string;
  qr_code: string;
  error: string;
}

interface DesktopLANShareParams {
  enabled: boolean;
  host: string;
  shelfIDs: string[];
}

export interface DesktopQRShelf {
  id: string;
  name: string;
  url: string;
}
export interface DesktopQRResult {
  name: string;
  url: string;
  status: string;
}

interface DesktopAppBinding {
  PreviewLANShareQR?: (text: string) => Promise<DesktopQRShelf[]>;
  ImportLANShareQR?: (text: string) => Promise<DesktopQRResult[]>;
  OpenBookFiles?: () => Promise<string[]>;
  ImportBookFromLocalPath?: (
    shelfID: string,
    localPath: string,
    folderParts: string[]
  ) => Promise<DesktopImportBookResult>;
  OpenShelfDirectory?: () => Promise<string>;
  OpenShelfInFinder?: (shelfID: string) => Promise<void>;
  PreviewShelfID?: (name: string) => Promise<DesktopShelfIDPreview>;
  OpenFolderDirectory?: (shelfID: string, folderParts: string[]) => Promise<void>;
  OpenBookDirectory?: (shelfID: string, bookID: string) => Promise<void>;
  OpenReader?: (shelfID: string, bookID: string, section: number) => Promise<void>;
  AddShelf?: (params: DesktopAddShelfParams) => Promise<void>;
  CheckShelfSource?: (params: DesktopShelfSourceParams) => Promise<ShelfSourceCheck>;
  RemoveShelf?: (shelfID: string) => Promise<void>;
  ListShelfDetails?: () => Promise<DesktopShelfDetails[]>;
  RetryShelf?: (shelfID: string) => Promise<void>;
  GetShelfDetails?: (shelfID: string) => Promise<DesktopShelfDetails>;
  ModifyShelf?: (params: DesktopModifyShelfParams) => Promise<void>;
  SaveBookContent?: (shelfID: string, bookID: string, suggestedName: string) => Promise<void>;
  OpenExternalURL?: (url: string) => Promise<void>;
  GetLANShare?: () => Promise<DesktopLANShareStatus>;
  SetLANShare?: (params: DesktopLANShareParams) => Promise<DesktopLANShareStatus>;
  RotateLANShareToken?: () => Promise<DesktopLANShareStatus>;
  ReadReadHistory?: () => Promise<string>;
  WriteReadHistory?: (doc: string) => Promise<void>;
  ReadReadingProgress?: () => Promise<string>;
  WriteReadingProgress?: (doc: string) => Promise<void>;
  ReadReadingStats?: () => Promise<string>;
  WriteReadingStats?: (doc: string) => Promise<void>;
  StageReadingProgress?: (
    shelfID: string,
    bookID: string,
    offset: number,
    at: number
  ) => Promise<void>;
}

type ReaderAppBinding = Pick<
  DesktopAppBinding,
  'ReadReadingProgress' | 'WriteReadingProgress' | 'StageReadingProgress'
>;

interface DesktopWindow extends Window {
  go?: {
    main?: {
      DesktopApp?: DesktopAppBinding;
      ReaderApp?: ReaderAppBinding;
    };
  };
}

function isDesktopRuntime(): boolean {
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

function normalizeFolderParts(folderPath: string): string[] {
  const trimmed = folderPath.trim();
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

export async function openDesktopShelfFolder(shelfID: string): Promise<void> {
  if (!isDesktopRuntime()) {
    return;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenShelfInFinder) {
    return;
  }

  await desktopApp.OpenShelfInFinder(shelfID);
}

export async function previewDesktopShelfID(name: string): Promise<DesktopShelfIDPreview> {
  const empty: DesktopShelfIDPreview = { id: '', defaultPath: '' };
  if (!isDesktopRuntime()) {
    return empty;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.PreviewShelfID) {
    return empty;
  }

  return (await desktopApp.PreviewShelfID(name)) ?? empty;
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

const READER_UNSUPPORTED_PLATFORM_CODE = 'reader_unsupported_platform';

export function isReaderUnsupportedPlatform(error: unknown): boolean {
  const message =
    error instanceof Error ? error.message : typeof error === 'string' ? error : String(error);
  return message.includes(READER_UNSUPPORTED_PLATFORM_CODE);
}

export async function openDesktopReader(bookID: string, section?: number): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenReader) {
    throw new Error('OpenReader binding not available');
  }

  const sectionArg =
    typeof section === 'number' && Number.isFinite(section) ? Math.trunc(section) : -1;
  await desktopApp.OpenReader(getActiveShelfID(), bookID, sectionArg);
}

export async function openDesktopFolder(folderPath: string): Promise<void> {
  if (!isDesktopRuntime()) {
    return;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenFolderDirectory) {
    return;
  }

  await desktopApp.OpenFolderDirectory(getActiveShelfID(), normalizeFolderParts(folderPath));
}

export async function addDesktopShelf(params: DesktopAddShelfParams): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.AddShelf) {
    throw new Error('AddShelf binding not available');
  }

  await desktopApp.AddShelf(params);
}

export async function checkDesktopShelfSource(params: DesktopShelfSourceParams): Promise<ShelfSourceCheck> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.CheckShelfSource) {
    throw new Error('CheckShelfSource binding not available');
  }

  return desktopApp.CheckShelfSource(params);
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

export async function modifyDesktopShelf(params: DesktopModifyShelfParams): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ModifyShelf) {
    throw new Error('ModifyShelf binding not available');
  }

  await desktopApp.ModifyShelf(params);
}

export function hasDesktopLANShareBinding(): boolean {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  return isDesktopRuntime() && Boolean(desktopApp?.GetLANShare && desktopApp?.SetLANShare);
}

export async function getDesktopLANShare(): Promise<DesktopLANShareStatus> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.GetLANShare) {
    throw new Error('GetLANShare binding not available');
  }

  return desktopApp.GetLANShare();
}

export async function setDesktopLANShare(
  params: DesktopLANShareParams
): Promise<DesktopLANShareStatus> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.SetLANShare) {
    throw new Error('SetLANShare binding not available');
  }

  return desktopApp.SetLANShare(params);
}

export async function rotateDesktopLANShareToken(): Promise<DesktopLANShareStatus> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.RotateLANShareToken) {
    throw new Error('RotateLANShareToken binding not available');
  }

  return desktopApp.RotateLANShareToken();
}

export async function importDesktopBookFromLocalPath(
  localPath: string,
  folderPath: string
): Promise<DesktopImportBookResult | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.ImportBookFromLocalPath) {
    return null;
  }

  return desktopApp.ImportBookFromLocalPath(
    getActiveShelfID(),
    localPath,
    normalizeFolderParts(folderPath)
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

function getReadingProgressBinding(): ReaderAppBinding | undefined {
  const main = (window as DesktopWindow).go?.main;
  const desktopApp = main?.DesktopApp;
  if (desktopApp?.ReadReadingProgress && desktopApp?.WriteReadingProgress) {
    return desktopApp;
  }
  const readerApp = main?.ReaderApp;
  if (readerApp?.ReadReadingProgress && readerApp?.WriteReadingProgress) {
    return readerApp;
  }
  return undefined;
}

export function hasDesktopReadingProgressBinding(): boolean {
  return getReadingProgressBinding() !== undefined;
}

export async function readDesktopReadingProgress(): Promise<string> {
  const binding = getReadingProgressBinding();
  if (!binding?.ReadReadingProgress) {
    throw new Error('ReadReadingProgress binding not available');
  }

  return (await binding.ReadReadingProgress()) ?? '';
}

export async function writeDesktopReadingProgress(doc: string): Promise<void> {
  const binding = getReadingProgressBinding();
  if (!binding?.WriteReadingProgress) {
    throw new Error('WriteReadingProgress binding not available');
  }

  await binding.WriteReadingProgress(doc);
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

export function stageDesktopReadingProgress(bookID: string, offset: number, at: number): void {
  if (typeof window === 'undefined') {
    return;
  }
  const binding = getReadingProgressBinding();
  if (!binding?.StageReadingProgress) {
    return;
  }
  void binding.StageReadingProgress(getActiveShelfID(), bookID, offset, at).catch(() => undefined);
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

export async function listDesktopShelfDetails(): Promise<DesktopShelfDetails[]> {
  const app = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!app?.ListShelfDetails) throw new Error('ListShelfDetails binding not available');
  return app.ListShelfDetails();
}

export async function retryDesktopShelf(id: string): Promise<void> {
  const app = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!app?.RetryShelf) throw new Error('RetryShelf binding not available');
  return app.RetryShelf(id);
}

export function hasDesktopShelfManagementBindings(): boolean {
  if (typeof window === 'undefined') return false;
  const app = (window as DesktopWindow).go?.main?.DesktopApp;
  return typeof app?.ListShelfDetails === 'function' && typeof app?.RetryShelf === 'function';
}

export async function previewDesktopLANShareQR(text: string): Promise<DesktopQRShelf[]> {
  const binding = (window as DesktopWindow).go?.main?.DesktopApp?.PreviewLANShareQR;
  if (!binding) throw new Error('PreviewLANShareQR binding not available');
  return binding(text);
}

export async function importDesktopLANShareQR(text: string): Promise<DesktopQRResult[]> {
  const binding = (window as DesktopWindow).go?.main?.DesktopApp?.ImportLANShareQR;
  if (!binding) throw new Error('ImportLANShareQR binding not available');
  return binding(text);
}
