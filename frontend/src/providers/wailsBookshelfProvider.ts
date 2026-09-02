import {
  addDesktopShelf,
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
import type { DesktopShelfDetails, DesktopShelfNamePreview } from '@/api/desktop';
import { getActiveShelfID } from '@/api/client';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import type { DesktopImportBookResult } from './bookshelfProvider';

export class WailsBookshelfProvider extends ServerBookshelfProvider {
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
      await this.addReadHistory(bookId);
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

  previewDesktopShelfID(name: string): Promise<DesktopShelfNamePreview> {
    return previewDesktopShelfID(name);
  }

  addDesktopShelf(
    name: string,
    libRoot: string,
    scanInterval: string,
    readOnly: boolean
  ): Promise<void> {
    return addDesktopShelf(name, libRoot, scanInterval, readOnly);
  }

  removeDesktopShelf(shelfID: string): Promise<void> {
    return removeDesktopShelf(shelfID);
  }

  getDesktopShelfDetails(shelfID: string): Promise<DesktopShelfDetails> {
    return getDesktopShelfDetails(shelfID);
  }

  modifyDesktopShelf(
    shelfID: string,
    name: string,
    scanInterval: string,
    readOnly: boolean
  ): Promise<void> {
    return modifyDesktopShelf(shelfID, name, scanInterval, readOnly);
  }

  saveBookContentToFile(bookId: string, suggestedName: string): Promise<void> {
    return saveDesktopBookContent(getActiveShelfID(), bookId, suggestedName);
  }
}
