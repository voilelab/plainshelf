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
  removeDesktopShelf,
  saveDesktopBookContent
} from '@/api/desktop';
import type { DesktopShelfDetails } from '@/api/desktop';
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

  openDesktopReader(bookId: string, section?: number): Promise<void> {
    return openDesktopReader(bookId, section);
  }

  openDesktopShelfDirectory(): Promise<string | null> {
    return openDesktopShelfDirectory();
  }

  addDesktopShelf(name: string, libRoot: string, scanInterval: string): Promise<void> {
    return addDesktopShelf(name, libRoot, scanInterval);
  }

  removeDesktopShelf(shelfID: string): Promise<void> {
    return removeDesktopShelf(shelfID);
  }

  getDesktopShelfDetails(shelfID: string): Promise<DesktopShelfDetails> {
    return getDesktopShelfDetails(shelfID);
  }

  modifyDesktopShelf(shelfID: string, name: string, scanInterval: string): Promise<void> {
    return modifyDesktopShelf(shelfID, name, scanInterval);
  }

  saveBookContentToFile(bookId: string, suggestedName: string): Promise<void> {
    return saveDesktopBookContent(getActiveShelfID(), bookId, suggestedName);
  }
}
