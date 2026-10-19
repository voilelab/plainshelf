import type { BookshelfProvider } from './bookshelfProvider';
import {
  openDesktopExternalURL,
  listDesktopShelfDetails,
  retryDesktopShelf,
  addDesktopShelf,
  checkDesktopShelfSource,
  getDesktopShelfDetails,
  importDesktopBookFromLocalPath,
  modifyDesktopShelf,
  openDesktopBookFiles,
  openDesktopBookFolder,
  openDesktopFolder,
  openDesktopReader,
  openDesktopShelfDirectory,
  openDesktopShelfFolder,
  previewDesktopShelfID,
  removeDesktopShelf,
  saveDesktopBookContent
} from '@/api/desktop';
import type {
  DesktopAddShelfParams,
  DesktopModifyShelfParams,
  DesktopShelfDetails,
  DesktopShelfIDPreview,
  DesktopShelfSourceParams
} from '@/api/desktop';
import { getActiveShelfID } from '@/api/client';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import type { DesktopImportBookResult } from './platformCapabilities';
import type { PlatformCapabilities } from './platformCapabilities';
import type { ShelfSourceCheck } from '@/utils/shelfSourceCheck';

export class WailsBookshelfProvider extends ServerBookshelfProvider {}

class WailsPlatform {
  constructor(private readonly provider: BookshelfProvider) {}
  openLocalBookFiles(): Promise<string[] | null> {
    return openDesktopBookFiles();
  }

  importBookFromLocalPath(
    localPath: string,
    folderPath: string
  ): Promise<DesktopImportBookResult | null> {
    return importDesktopBookFromLocalPath(localPath, folderPath);
  }

  openDesktopFolder(folderPath: string): Promise<void> {
    return openDesktopFolder(folderPath);
  }

  openDesktopBookFolder(bookId: string): Promise<void> {
    return openDesktopBookFolder(bookId);
  }

  async openDesktopReader(bookId: string, section?: number): Promise<void> {
    await openDesktopReader(bookId, section);
    // The standalone reader records its own read history in its WebView storage,
    // which this desktop app cannot read, so its "recent reading" would stay
    // empty on the default shell-out path. Write the entry from the launching
    // side once the reader has actually started.
    //
    // Swallow every failure: the launch already succeeded, and letting it reach
    // useReaderLaunch would be read as a launch failure — it would then pop a
    // toast and open a second, in-app reader. A try/catch, not `.catch()`,
    // because addReadHistory throws synchronously when no shelf is selected
    // (requireHistoryKey() runs before the promise is returned), which a
    // trailing `.catch()` would miss.
    try {
      await this.provider.addReadHistory(bookId);
    } catch (err) {
      console.warn('Failed to update read history', err);
    }
  }

  openDesktopShelfDirectory(): Promise<string | null> {
    return openDesktopShelfDirectory();
  }

  openDesktopShelfFolder(shelfID: string): Promise<void> {
    return openDesktopShelfFolder(shelfID);
  }

  previewDesktopShelfID(name: string): Promise<DesktopShelfIDPreview> {
    return previewDesktopShelfID(name);
  }

  addDesktopShelf(params: DesktopAddShelfParams): Promise<void> {
    return addDesktopShelf(params);
  }

  checkDesktopShelfSource(params: DesktopShelfSourceParams): Promise<ShelfSourceCheck> {
    return checkDesktopShelfSource(params);
  }

  removeDesktopShelf(shelfID: string): Promise<void> {
    return removeDesktopShelf(shelfID);
  }

  getDesktopShelfDetails(shelfID: string): Promise<DesktopShelfDetails> {
    return getDesktopShelfDetails(shelfID);
  }

  modifyDesktopShelf(params: DesktopModifyShelfParams): Promise<void> {
    return modifyDesktopShelf(params);
  }

  saveBookContentToFile(bookId: string, suggestedName: string): Promise<void> {
    return saveDesktopBookContent(getActiveShelfID(), bookId, suggestedName);
  }
}

export function createWailsPlatform(provider: BookshelfProvider): PlatformCapabilities {
  const host = new WailsPlatform(provider);
  return {
    shelves: {
      listShelfDetails: listDesktopShelfDetails,
      retryShelf: retryDesktopShelf,
      previewShelfID: host.previewDesktopShelfID.bind(host),
      addShelf: host.addDesktopShelf.bind(host),
      checkShelfSource: host.checkDesktopShelfSource.bind(host),
      getShelfDetails: host.getDesktopShelfDetails.bind(host),
      modifyShelf: host.modifyDesktopShelf.bind(host),
      removeShelf: host.removeDesktopShelf.bind(host)
    },
    openLocalBookFiles: host.openLocalBookFiles.bind(host),
    importBookFromLocalPath: host.importBookFromLocalPath.bind(host),
    openFolder: host.openDesktopFolder.bind(host),
    openBookFolder: host.openDesktopBookFolder.bind(host),
    openReader: host.openDesktopReader.bind(host),
    chooseShelfDirectory: host.openDesktopShelfDirectory.bind(host),
    openShelfFolder: host.openDesktopShelfFolder.bind(host),
    saveBookContentToFile: host.saveBookContentToFile.bind(host),
    openExternalUrl: openDesktopExternalURL
  };
}
