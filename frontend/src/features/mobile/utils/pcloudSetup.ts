import { PCloudClient } from '@/api/pcloud/client';
import { LocalShelf } from '@/providers/localShelf';
import type { PCloudFolderClient } from '@/features/mobile/composables/usePCloudFolderBrowser';

/**
 * The picker's client: levels are listed by Go, which knows the shelf layout;
 * a typed path is resolved here, which needs only pCloud's folder names.
 */
export function pcloudFolderClient(host: string, accessToken: string): PCloudFolderClient {
  return {
    // The bridge cannot cancel; the browser drops a reply it no longer wants.
    async listFolder(folderid) {
      const { folders, isShelf } = await LocalShelf.listPCloudFolder({
        host,
        token: accessToken,
        folderId: folderid
      });
      return { folders, isShelf };
    },
    resolveFolderTrail: (path, signal) =>
      new PCloudClient({ host, accessToken }).resolveFolderTrail(path, signal)
  };
}

/** The book count of the shelf at `folder`, or null when it is not a shelf. */
export async function checkPCloudShelf(
  host: string,
  accessToken: string,
  folder: string
): Promise<number | null> {
  const { isShelf, books } = await LocalShelf.checkPCloudShelf({ host, token: accessToken, folder });
  return isShelf ? books : null;
}
