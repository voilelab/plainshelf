import { PCloudClient } from '@/api/pcloud/client';
import { LocalShelf } from '@/providers/localShelf';
import type { PCloudFolderClient } from '@/features/mobile/composables/usePCloudFolderBrowser';

export function pcloudFolderClient(host: string, accessToken: string): PCloudFolderClient {
  return {
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

export async function checkPCloudShelf(
  host: string,
  accessToken: string,
  folder: string
): Promise<number | null> {
  const { isShelf, books } = await LocalShelf.checkPCloudShelf({ host, token: accessToken, folder });
  return isShelf ? books : null;
}
