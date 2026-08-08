import type { BookshelfProvider } from './bookshelfProvider';
import { isMobileRuntime, isWailsRuntime } from './runtime';
import { getMobileConnectionConfig, isConnectionConfigured } from './mobileConfig';
import type { MobileConnectionConfig } from './mobileConfig';
import { MobileBookshelfProvider } from './mobileBookshelfProvider';
import { FilesystemMobileBookCache } from './filesystemMobileBookCache';
import { PCloudBookshelfProvider } from './pcloudBookshelfProvider';
import { PCloudClient } from '@/api/pcloud/client';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import { FilesystemShelfSnapshotStore } from './shelfSnapshotStore';
import { WailsBookshelfProvider } from './wailsBookshelfProvider';

let provider: BookshelfProvider | null = null;

/**
 * Picks what the mobile shell reads from.
 *
 * A pCloud connection is only selected once it is complete. The provider is
 * built during bootstrap, before the router has had a chance to send an
 * unconfigured install to the connection page, and PCloudClient refuses to
 * construct without a token — so an incomplete configuration would throw here
 * and take the whole app down instead of showing the setup form.
 */
function createMobileSource(config: MobileConnectionConfig | null): BookshelfProvider {
  if (config?.mode !== 'pcloud' || !isConnectionConfigured(config)) {
    return new ServerBookshelfProvider();
  }

  return new PCloudBookshelfProvider({
    client: new PCloudClient({ host: config.pcloudHost, accessToken: config.pcloudAccessToken }),
    shelfRoot: config.pcloudShelfRoot,
    // Persisted for the same reason downloads are (see below): without it the
    // shelf would be walked once per app launch, which is what manual updating
    // exists to avoid.
    snapshotStore: new FilesystemShelfSnapshotStore()
  });
}

export function createBookshelfProvider(): BookshelfProvider {
  if (isWailsRuntime()) {
    return new WailsBookshelfProvider();
  }

  if (isMobileRuntime()) {
    // Persist downloads and reading progress across app restarts.
    // Filesystem-backed (Directory.Data): app-private files are exempt from
    // the WebView's best-effort storage eviction, unlike IndexedDB.
    return new MobileBookshelfProvider(
      createMobileSource(getMobileConnectionConfig()),
      new FilesystemMobileBookCache()
    );
  }

  return new ServerBookshelfProvider();
}

export function getBookshelfProvider(): BookshelfProvider {
  if (!provider) {
    provider = createBookshelfProvider();
  }
  return provider;
}

export type { BookshelfProvider } from './bookshelfProvider';
export { isMobileRuntime, isWailsRuntime } from './runtime';
export type { CachedBookManifest, MobileBookCache } from './mobileBookCache';
export { InMemoryMobileBookCache } from './mobileBookCache';
export { FilesystemMobileBookCache } from './filesystemMobileBookCache';
