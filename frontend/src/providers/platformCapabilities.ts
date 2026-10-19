import type {
  DesktopAddShelfParams, DesktopModifyShelfParams, DesktopShelfDetails,
  DesktopShelfIDPreview, DesktopShelfSourceParams
} from '@/api/desktop';
import type { ShelfSourceCheck } from '@/utils/shelfSourceCheck';
import { getShell } from './shell';
import { isWailsRuntime } from './runtime';
import { createWailsPlatform } from './wailsBookshelfProvider';
import { getBookshelfProvider } from './index';

export interface DesktopImportBookResult {
  path?: string;
  id?: string;
  error?: string;
}

export interface StorageEstimateResult {
  supported: boolean;
  usage?: number;
  quota?: number;
}

/** Device operations supplied by the host; never part of the shelf HTTP API. */
export interface PlatformCapabilities {
  shelves?: {
    previewShelfID(name: string): Promise<DesktopShelfIDPreview>;
    addShelf(params: DesktopAddShelfParams): Promise<void>;
    checkShelfSource(params: DesktopShelfSourceParams): Promise<ShelfSourceCheck>;
    listShelfDetails?(): Promise<DesktopShelfDetails[]>;
    retryShelf?(id: string): Promise<void>;
    getShelfDetails(id: string): Promise<DesktopShelfDetails>;
    modifyShelf(params: DesktopModifyShelfParams): Promise<void>;
    removeShelf(id: string): Promise<void>;
  };
  openLocalBookFiles?(): Promise<string[] | null>;
  importBookFromLocalPath?(path: string, folder: string): Promise<DesktopImportBookResult | null>;
  openFolder?(path: string): Promise<void>;
  openBookFolder?(id: string): Promise<void>;
  openShelfFolder?(id: string): Promise<void>;
  chooseShelfDirectory?(): Promise<string | null>;
  saveBookContentToFile?(id: string, name: string): Promise<string | void>;
  openReader?(id: string, section?: number): Promise<void>;
  openExternalUrl?(url: string): Promise<void>;
  getStorageEstimate?(): Promise<StorageEstimateResult>;
  coversMustBeFetchedAsBlob?: boolean;
}

/** Web and wasm use the same browser host; wasm's shelf toolbar remains outside the app. */
export function createBrowserPlatform(): PlatformCapabilities {
  return {
    async openExternalUrl(url) {
      // noopener can return null even when the tab opened successfully.
      window.open(url, '_blank', 'noopener,noreferrer');
    }
  };
}

let desktop: PlatformCapabilities | undefined;
const browser = createBrowserPlatform();
export function getPlatformCapabilities(): PlatformCapabilities {
  const fromShell = getShell()?.platform;
  if (fromShell) return fromShell;
  if (isWailsRuntime()) {
    desktop ??= createWailsPlatform(getBookshelfProvider());
    return desktop;
  }
  return browser;
}
