import { PCloudClient } from '@/api/pcloud/client';
import type { BookshelfProvider } from '@/providers/bookshelfProvider';
import { FilesystemMobileBookCache } from '@/providers/filesystemMobileBookCache';
import { FilesystemMobileCoverCache } from '@/providers/filesystemMobileCoverCache';
import { MobileBookshelfProvider } from '@/providers/mobileBookshelfProvider';
import {
  getActiveShelfEntry,
  getActiveShelfEntryToken,
  initMobileConfig,
  isShelfEntryConfigured,
  type ShelfEntry
} from '@/providers/mobileConfig';
import { PCloudBookshelfProvider } from '@/providers/pcloudBookshelfProvider';
import { registerShell } from '@/providers/shell';
import { createMobileDeviceDocumentStorage } from './deviceDocumentStorage';
import { activeMobileShelfInfo } from './shelfInfo';
import { createMobileShelfPicker } from './shelfPicker';
import { ServerBookshelfProvider } from '@/providers/serverBookshelfProvider';
import { FilesystemShelfSnapshotStore } from '@/providers/shelfSnapshotStore';

/**
 * Picks what the mobile shell reads from, for the shelf the device has active.
 *
 * A pCloud entry is only selected once it is complete. The provider is built
 * during bootstrap, before the router has had a chance to send an unconfigured
 * install to the shelf list, and PCloudClient refuses to construct without a
 * token — so an incomplete entry, or none at all, would throw here and take the
 * whole app down instead of showing the setup form.
 */
function createMobileSource(entry: ShelfEntry | null): BookshelfProvider {
  const accessToken = getActiveShelfEntryToken();
  if (entry?.type !== 'pcloud' || !isShelfEntryConfigured(entry) || !accessToken) {
    return new ServerBookshelfProvider();
  }

  return new PCloudBookshelfProvider({
    client: new PCloudClient({ host: entry.pcloudHost, accessToken }),
    shelfRoot: entry.pcloudShelfRoot,
    // Persisted for the same reason downloads are: without it the shelf would
    // be walked once per app launch, which is what manual updating exists to
    // avoid.
    snapshotStore: new FilesystemShelfSnapshotStore()
  });
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

  registerShell({
    activeShelfInfo: activeMobileShelfInfo,
    createShelfPicker: createMobileShelfPicker,
    createDeviceDocumentStorage: createMobileDeviceDocumentStorage,

    createProvider: () =>
      // Persist downloads and reading progress across app restarts, in
      // Directory.Data: app-private files are exempt from the WebView's
      // best-effort storage eviction, which the browser storage APIs are not.
      new MobileBookshelfProvider(
        createMobileSource(getActiveShelfEntry()),
        new FilesystemMobileBookCache(),
        undefined,
        new FilesystemMobileCoverCache()
      )
  });
}
