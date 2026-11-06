import { getActiveShelfID } from './client';
import { isWailsRuntime } from '@/providers/runtime';
import { callWails, wailsHost } from './wailsBindings';
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
  PreviewLANShareQR: (text: string) => Promise<DesktopQRShelf[]>;
  ImportLANShareQR: (text: string) => Promise<DesktopQRResult[]>;
  OpenBookFiles: () => Promise<string[]>;
  ImportBookFromLocalPath: (
    shelfID: string,
    localPath: string,
    folderParts: string[]
  ) => Promise<DesktopImportBookResult>;
  OpenShelfDirectory: () => Promise<string>;
  OpenShelfInFinder: (shelfID: string) => Promise<void>;
  PreviewShelfID: (name: string) => Promise<DesktopShelfIDPreview>;
  OpenFolderDirectory: (shelfID: string, folderParts: string[]) => Promise<void>;
  OpenBookDirectory: (shelfID: string, bookID: string) => Promise<void>;
  OpenReader: (shelfID: string, bookID: string, section: number) => Promise<void>;
  AddShelf: (params: DesktopAddShelfParams) => Promise<void>;
  CheckShelfSource: (params: DesktopShelfSourceParams) => Promise<ShelfSourceCheck>;
  RemoveShelf: (shelfID: string) => Promise<void>;
  ListShelfDetails: () => Promise<DesktopShelfDetails[]>;
  RetryShelf: (shelfID: string) => Promise<void>;
  GetShelfDetails: (shelfID: string) => Promise<DesktopShelfDetails>;
  ModifyShelf: (params: DesktopModifyShelfParams) => Promise<void>;
  SaveBookContent: (shelfID: string, bookID: string, suggestedName: string) => Promise<void>;
  OpenExternalURL: (url: string) => Promise<void>;
  GetLANShare: () => Promise<DesktopLANShareStatus>;
  SetLANShare: (params: DesktopLANShareParams) => Promise<DesktopLANShareStatus>;
  RotateLANShareToken: () => Promise<DesktopLANShareStatus>;
  ReadReadHistory: () => Promise<string>;
  RecordReadHistory: (shelfKey: string, bookID: string) => Promise<void>;
  ClearReadHistory: (shelfKey: string) => Promise<void>;
  SetReadHistoryLimit: (limit: number) => Promise<void>;
  ReadReadingProgress: () => Promise<string>;
  WriteReadingProgress: (doc: string) => Promise<void>;
  ReadReadingStats: () => Promise<string>;
  AddReadingSeconds: (key: string, date: string, seconds: number) => Promise<void>;
  StageReadingProgress: (
    shelfID: string,
    bookID: string,
    offset: number,
    at: number
  ) => Promise<void>;
}

type DeviceDocumentBinding = Pick<
  DesktopAppBinding,
  | 'ReadReadHistory'
  | 'RecordReadHistory'
  | 'ClearReadHistory'
  | 'SetReadHistoryLimit'
  | 'ReadReadingProgress'
  | 'WriteReadingProgress'
  | 'ReadReadingStats'
  | 'AddReadingSeconds'
  | 'StageReadingProgress'
>;

function callDesktop<M extends keyof DesktopAppBinding>(
  method: M,
  ...args: Parameters<DesktopAppBinding[M]>
): ReturnType<DesktopAppBinding[M]> {
  return callWails<DesktopAppBinding, M>('DesktopApp', method, ...args);
}

function inDesktopApp(): boolean {
  return wailsHost() === 'DesktopApp';
}

export async function openDesktopBookFiles(): Promise<string[] | null> {
  if (!inDesktopApp()) {
    return null;
  }

  return callDesktop('OpenBookFiles');
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
  if (!inDesktopApp()) {
    return null;
  }

  const dir = await callDesktop('OpenShelfDirectory');
  return dir || null;
}

export async function openDesktopShelfFolder(shelfID: string): Promise<void> {
  if (!inDesktopApp()) {
    return;
  }

  await callDesktop('OpenShelfInFinder', shelfID);
}

export async function previewDesktopShelfID(name: string): Promise<DesktopShelfIDPreview> {
  const empty: DesktopShelfIDPreview = { id: '', defaultPath: '' };
  if (!inDesktopApp()) {
    return empty;
  }

  return (await callDesktop('PreviewShelfID', name)) ?? empty;
}

export async function openDesktopBookFolder(bookID: string): Promise<void> {
  if (!inDesktopApp()) {
    return;
  }

  await callDesktop('OpenBookDirectory', getActiveShelfID(), bookID);
}

export async function openDesktopReader(bookID: string, section?: number): Promise<void> {
  const sectionArg =
    typeof section === 'number' && Number.isFinite(section) ? Math.trunc(section) : -1;
  await callDesktop('OpenReader', getActiveShelfID(), bookID, sectionArg);
}

export async function openDesktopFolder(folderPath: string): Promise<void> {
  if (!inDesktopApp()) {
    return;
  }

  await callDesktop('OpenFolderDirectory', getActiveShelfID(), normalizeFolderParts(folderPath));
}

export async function addDesktopShelf(params: DesktopAddShelfParams): Promise<void> {
  await callDesktop('AddShelf', params);
}

export async function checkDesktopShelfSource(params: DesktopShelfSourceParams): Promise<ShelfSourceCheck> {
  return callDesktop('CheckShelfSource', params);
}

export async function removeDesktopShelf(shelfID: string): Promise<void> {
  await callDesktop('RemoveShelf', shelfID);
}

export async function getDesktopShelfDetails(shelfID: string): Promise<DesktopShelfDetails> {
  return callDesktop('GetShelfDetails', shelfID);
}

export async function modifyDesktopShelf(params: DesktopModifyShelfParams): Promise<void> {
  await callDesktop('ModifyShelf', params);
}

export function hasDesktopLANShareBinding(): boolean {
  return inDesktopApp();
}

export async function getDesktopLANShare(): Promise<DesktopLANShareStatus> {
  return callDesktop('GetLANShare');
}

export async function setDesktopLANShare(
  params: DesktopLANShareParams
): Promise<DesktopLANShareStatus> {
  return callDesktop('SetLANShare', params);
}

export async function rotateDesktopLANShareToken(): Promise<DesktopLANShareStatus> {
  return callDesktop('RotateLANShareToken');
}

export async function importDesktopBookFromLocalPath(
  localPath: string,
  folderPath: string
): Promise<DesktopImportBookResult | null> {
  if (!inDesktopApp()) {
    return null;
  }

  return callDesktop(
    'ImportBookFromLocalPath',
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
  await callDesktop('SaveBookContent', shelfID, bookID, suggestedName);
}

function inWailsApp(): boolean {
  return wailsHost() !== null;
}

function callDeviceDocument<M extends keyof DeviceDocumentBinding>(
  method: M,
  ...args: Parameters<DeviceDocumentBinding[M]>
): ReturnType<DeviceDocumentBinding[M]> {
  return callWails<DeviceDocumentBinding, M>(wailsHost() ?? 'DesktopApp', method, ...args);
}

export function hasDesktopReadHistoryBinding(): boolean {
  return inWailsApp();
}

export async function readDesktopReadHistory(): Promise<string> {
  return (await callDeviceDocument('ReadReadHistory')) ?? '';
}

export async function recordDesktopReadHistory(shelfKey: string, bookID: string): Promise<void> {
  await callDeviceDocument('RecordReadHistory', shelfKey, bookID);
}

export async function clearDesktopReadHistory(shelfKey: string): Promise<void> {
  await callDeviceDocument('ClearReadHistory', shelfKey);
}

export async function setDesktopReadHistoryLimit(limit: number): Promise<void> {
  await callDeviceDocument('SetReadHistoryLimit', limit);
}

export function hasDesktopReadingProgressBinding(): boolean {
  return inWailsApp();
}

export async function readDesktopReadingProgress(): Promise<string> {
  return (await callDeviceDocument('ReadReadingProgress')) ?? '';
}

export async function writeDesktopReadingProgress(doc: string): Promise<void> {
  await callDeviceDocument('WriteReadingProgress', doc);
}

export function hasDesktopReadingStatsBinding(): boolean {
  return inWailsApp();
}

export async function readDesktopReadingStats(): Promise<string> {
  return (await callDeviceDocument('ReadReadingStats')) ?? '';
}

export async function addDesktopReadingSeconds(
  key: string,
  date: string,
  seconds: number
): Promise<void> {
  await callDeviceDocument('AddReadingSeconds', key, date, seconds);
}

export function stageDesktopReadingProgress(bookID: string, offset: number, at: number): void {
  if (!inWailsApp()) {
    return;
  }
  void callDeviceDocument('StageReadingProgress', getActiveShelfID(), bookID, offset, at).catch(
    () => undefined
  );
}

export async function openDesktopExternalURL(url: string): Promise<void> {
  if (!isWailsRuntime()) {
    return;
  }

  await callDesktop('OpenExternalURL', url);
}

export async function listDesktopShelfDetails(): Promise<DesktopShelfDetails[]> {
  return callDesktop('ListShelfDetails');
}

export async function retryDesktopShelf(id: string): Promise<void> {
  await callDesktop('RetryShelf', id);
}

export function hasDesktopShelfManagementBindings(): boolean {
  return inDesktopApp();
}

export async function previewDesktopLANShareQR(text: string): Promise<DesktopQRShelf[]> {
  return callDesktop('PreviewLANShareQR', text);
}

export async function importDesktopLANShareQR(text: string): Promise<DesktopQRResult[]> {
  return callDesktop('ImportLANShareQR', text);
}
