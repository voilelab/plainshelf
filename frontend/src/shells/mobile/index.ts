import { listServerShelves } from '@/api/shelves';
import { createMobilePlatform } from './platform';
import { setApiTransport } from '@/api/client';
import { FilesystemMobileBookCache } from '@/providers/filesystemMobileBookCache';
import { FilesystemMobileCoverCache } from '@/providers/filesystemMobileCoverCache';
import {
  EmbeddedPCloudReader,
  EMBEDDED_PCLOUD_SHELF_ID,
  embeddedPCloudFetch,
  openEmbeddedPCloudShelf
} from '@/providers/embeddedPCloud';
import { localShelfFetch } from '@/providers/localShelf';
import { MobileBookshelfProvider } from '@/providers/mobileBookshelfProvider';
import {
  getActiveShelfEntry,
  getActiveShelfEntryToken,
  initMobileConfig,
  isShelfEntryConfigured,
  type DavShelfEntry,
  type PCloudShelfEntry,
  type ShelfEntry
} from '@/providers/mobileConfig';
import { shelfSourceNeedsSecret } from '@/utils/shelfSourceCheck';
import { registerShell } from '@/providers/shell';
import { createMobileDeviceDocumentStorage } from './deviceDocumentStorage';
import { activeMobileShelfInfo } from './shelfInfo';
import { installMobileRouterGuards } from './routerGuard';
import { createMobileShelfPicker } from './shelfPicker';
import { ServerBookshelfProvider } from '@/providers/serverBookshelfProvider';

function isUsableEmbeddedEntry(entry: ShelfEntry | null): entry is PCloudShelfEntry | DavShelfEntry {
  if (!entry || !isShelfEntryConfigured(entry)) {
    return false;
  }
  if (entry.type === 'dav') {
    return !shelfSourceNeedsSecret(entry.davSource, entry.davUsername) || Boolean(getActiveShelfEntryToken());
  }
  return entry.type === 'pcloud' && Boolean(getActiveShelfEntryToken());
}

export async function installMobileShell(): Promise<void> {
  await initMobileConfig();
  const active = getActiveShelfEntry();
  const isLocal = active?.type === 'local';
  const wrapsRemote = !isLocal && !isUsableEmbeddedEntry(active);
  if (isLocal) {
    setApiTransport({ fetch: localShelfFetch, allowMobileWrites: true });
  }
  if (isUsableEmbeddedEntry(active)) {
    void openEmbeddedPCloudShelf(active);
    setApiTransport({ fetch: embeddedPCloudFetch, allowMobileWrites: false });
  }

  registerShell({
    platform: createMobilePlatform(),
    activeShelfInfo: activeMobileShelfInfo,
    async loadActiveShelfInfo(shelf) {
      try {
        const listed = await listServerShelves();
        const engineID = isUsableEmbeddedEntry(active) ? EMBEDDED_PCLOUD_SHELF_ID : shelf.id;
        const engine = listed.find((item) => item.id === engineID);
        return {
          ...shelf, ...engine, id: shelf.id, name: shelf.name,
          capabilities: { ...engine?.capabilities, ...(wrapsRemote ? { download: true } : {}) },
          offline: engine?.offline
        };
      } catch {
        return { ...shelf, capabilities: { download: wrapsRemote } };
      }
    },
    createShelfPicker: createMobileShelfPicker,
    installRouterGuards: installMobileRouterGuards,
    createDeviceDocumentStorage: createMobileDeviceDocumentStorage,

    createProvider: () => {
      if (isLocal) {
        return new ServerBookshelfProvider();
      }
      if (isUsableEmbeddedEntry(getActiveShelfEntry())) {
        return new EmbeddedPCloudReader();
      }
      // Capacitor Filesystem 8.1.3 Directory.Data uses native app-private files, separate from WebView origin storage.
      return new MobileBookshelfProvider(
        new ServerBookshelfProvider(),
        new FilesystemMobileBookCache(),
        undefined,
        new FilesystemMobileCoverCache()
      );
    }
  });
}
