import { setApiTransport } from '@/api/client';
import { FilesystemMobileBookCache } from '@/providers/filesystemMobileBookCache';
import { FilesystemMobileCoverCache } from '@/providers/filesystemMobileCoverCache';
import {
  EmbeddedPCloudReader,
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

/**
 * Whether the active entry is a pCloud or dav shelf the embedded server can
 * open. The provider is built during bootstrap, before the router has had a
 * chance to send an unconfigured install to the shelf list, so an incomplete
 * entry, or one whose token is gone, falls back to the server path and the
 * setup form.
 */
function isUsableEmbeddedEntry(entry: ShelfEntry | null): entry is PCloudShelfEntry | DavShelfEntry {
  if (!entry || !isShelfEntryConfigured(entry)) {
    return false;
  }
  if (entry.type === 'dav') {
    return !shelfSourceNeedsSecret(entry.davSource) || Boolean(getActiveShelfEntryToken());
  }
  return entry.type === 'pcloud' && Boolean(getActiveShelfEntryToken());
}

/**
 * Brings up the native mobile shell.
 *
 * Restores the saved server URL, token and shelf before registering, so the
 * provider this shell hands out is already pointed at the right shelf and the
 * first API call has a configured client.
 *
 * This module is the only entry point into the mobile stack. Everything it
 * pulls in — the mobile provider, its on-device caches, the pCloud client —
 * reaches the bundle through here and through nothing else, which is what
 * keeps that code out of the web and desktop builds.
 */
export async function installMobileShell(): Promise<void> {
  await initMobileConfig();
  const active = getActiveShelfEntry();
  const isLocal = active?.type === 'local';
  if (isLocal) {
    setApiTransport({ fetch: localShelfFetch, allowMobileWrites: true });
  }
  if (isUsableEmbeddedEntry(active)) {
    // Read through the embedded Go server, still read-only. Not awaited; see
    // openEmbeddedPCloudShelf. After a failed open the reads fail like an
    // unreachable shelf and fall back to the downloads.
    void openEmbeddedPCloudShelf(active);
    setApiTransport({ fetch: embeddedPCloudFetch, allowMobileWrites: false });
  }

  registerShell({
    activeShelfInfo: activeMobileShelfInfo,
    createShelfPicker: createMobileShelfPicker,
    installRouterGuards: installMobileRouterGuards,
    createDeviceDocumentStorage: createMobileDeviceDocumentStorage,

    createProvider: () => {
      // A local shelf is already on the device: it is read and written in
      // place, with nothing to download and nothing to keep offline.
      if (isLocal) {
        return new ServerBookshelfProvider();
      }
      // Persist downloads and reading progress across app restarts, in
      // Directory.Data: app-private files are exempt from the WebView's
      // best-effort storage eviction, which the browser storage APIs are not.
      return new MobileBookshelfProvider(
        isUsableEmbeddedEntry(getActiveShelfEntry()) ? new EmbeddedPCloudReader() : new ServerBookshelfProvider(),
        new FilesystemMobileBookCache(),
        undefined,
        new FilesystemMobileCoverCache()
      );
    }
  });
}
