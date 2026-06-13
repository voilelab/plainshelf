import {
  addDesktopShelf,
  importDesktopBooksFromLocalPaths,
  openDesktopBookFiles,
  openDesktopShelfDirectory,
  removeDesktopShelf
} from '../api/desktop';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import type { DesktopImportBookResult } from './bookshelfProvider';

export class WailsBookshelfProvider extends ServerBookshelfProvider {
  openLocalBookFiles(): Promise<string[] | null> {
    return openDesktopBookFiles();
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

  addDesktopShelf(name: string, libRoot: string): Promise<void> {
    return addDesktopShelf(name, libRoot);
  }

  removeDesktopShelf(shelfID: string): Promise<void> {
    return removeDesktopShelf(shelfID);
  }
}
