import {
  addDesktopShelf,
  getDesktopShelfDetails,
  importDesktopBooksFromLocalPaths,
  modifyDesktopShelf,
  openDesktopBookFolder,
  openDesktopBookFiles,
  openDesktopShelfDirectory,
  removeDesktopShelf
} from '../api/desktop';
import type { DesktopShelfDetails } from '../api/desktop';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import type { DesktopImportBookResult } from './bookshelfProvider';

export class WailsBookshelfProvider extends ServerBookshelfProvider {
  openLocalBookFiles(): Promise<string[] | null> {
    return openDesktopBookFiles();
  }

  openDesktopBookFolder(bookId: string): Promise<void> {
    return openDesktopBookFolder(bookId);
  }

  importBooksFromLocalPaths(
    localPaths: string[],
    layerPath: string
  ): Promise<DesktopImportBookResult[] | null> {
    return importDesktopBooksFromLocalPaths(localPaths, layerPath);
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
}
