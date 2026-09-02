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
  book_check_interval: string;
  read_only: boolean;
}

// Keys match the json tags on desktop.AddShelfParams / desktop.ModifyShelfParams,
// which is how Wails unmarshals the object into the Go struct. Adding a per-shelf
// setting means adding a field here, never another positional binding argument.
export interface DesktopAddShelfParams {
  name: string;
  libRoot: string;
  scanInterval: string;
  bookCheckInterval: string;
  readOnly: boolean;
}

/**
 * What the add-shelf form previews for a typed name: the id the shelf would get
 * and the directory it would be created in if the user picks none.
 */
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

interface DesktopAppBinding {
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
  RemoveShelf?: (shelfID: string) => Promise<void>;
  GetShelfDetails?: (shelfID: string) => Promise<DesktopShelfDetails>;
  ModifyShelf?: (params: DesktopModifyShelfParams) => Promise<void>;
  SaveBookContent?: (shelfID: string, bookID: string, suggestedName: string) => Promise<void>;
  OpenExternalURL?: (url: string) => Promise<void>;
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

// The standalone reader binds a ReaderApp struct (window.go.main.ReaderApp)
// exposing only the reading-progress methods, so it can persist progress into
// the same file the desktop app uses instead of WebView localStorage.
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

// Reveals a shelf's lib_root in the host file explorer so a first-time user can
// find where their books live. No-op off the desktop or when the binding is
// missing, matching the other open-folder helpers.
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

// Returns the shelf id AddShelf would assign to a shelf named `name` right now,
// including any uniqueness suffix, plus the directory such a shelf would be
// created in when the user picks none — so the add-shelf form can show both
// live. Both fields are '' off the desktop, when the binding is missing, or for
// an empty name; the callers treat '' as "no preview".
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

// Stable token the desktop backend embeds in the OpenReader rejection on
// non-macOS platforms, where no standalone reader exists. Kept in sync with
// readerUnsupportedPlatformCode in desktop/app.go so the caller can tell "this
// platform has no standalone reader" apart from a macOS launch failure and word
// its in-app fallback notice accordingly.
export const READER_UNSUPPORTED_PLATFORM_CODE = 'reader_unsupported_platform';

// True when a rejected openDesktopReader call is the non-macOS "unsupported
// platform" case rather than a macOS launch failure (reader not installed, or
// the launch itself failed). Matches the backend error by its stable code token,
// which survives the util.NewError function-name prefix, so the substring check
// stays reliable.
//
// Wails rejects a bound-method promise with the Go error's message *string*, not
// an Error, so the real desktop path never produces an Error here; the in-app
// fallback wraps it in one. Normalize both shapes before matching.
export function isReaderUnsupportedPlatform(error: unknown): boolean {
  const message =
    error instanceof Error ? error.message : typeof error === 'string' ? error : String(error);
  return message.includes(READER_UNSUPPORTED_PLATFORM_CODE);
}

// Opens a book in the standalone reader's own window. Unlike the folder/finder
// helpers this rejects when the binding is missing, so the caller can fall back
// to the in-app reader rather than silently doing nothing.
//
// section is the reader section index to open at (a chapter "read" action); an
// undefined or non-finite section becomes -1, which the reader treats as "open at
// the restored progress" — the default read.
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

  // normalizeFolderParts splits by '/', trims each segment, and drops empties
  // before passing the folder path into the desktop binding.
  await desktopApp.OpenFolderDirectory(getActiveShelfID(), normalizeFolderParts(folderPath));
}

export async function addDesktopShelf(params: DesktopAddShelfParams): Promise<void> {
  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.AddShelf) {
    throw new Error('AddShelf binding not available');
  }

  await desktopApp.AddShelf(params);
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

// Imports a single host-path book. The frontend calls it once per selected file
// so the shared import executor can step through the batch, reporting the same
// N/M progress and file-boundary abort as the browser upload path. Returns null
// off the desktop or when the binding is missing, so the caller can fall back to
// the browser file-input modal.
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

// The desktop client and the standalone reader both persist reading progress
// through native bindings into the same reading_progress.json, but under
// different struct names (window.go.main.DesktopApp vs .ReaderApp). Resolve
// whichever one exposes the progress methods so the reader also stores progress
// in the shared file rather than falling back to WebView localStorage.
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

// Stages the reader's latest position with the native shell so it can be written
// to the shared file when the window closes — covering the seconds since the
// last autosave that a close would otherwise drop. The desktop app and the
// standalone reader both expose it (getReadingProgressBinding resolves whichever
// is present); it is a no-op off them.
//
// Fire-and-forget by design: it is called on every position change, so it must
// not block scrolling, and a dropped stage is covered by the next one or by the
// interval autosave. Errors are swallowed for the same reason.
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
