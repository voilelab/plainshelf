import { addDesktopShelf, importDesktopBooksFromLocalPaths, openDesktopBookFiles, openDesktopShelfDirectory } from '../api/desktop';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import type { DesktopImportBookResult } from './bookshelfProvider';
import type { DesktopShelfInfo } from '../api/desktop';

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

  openShelfDirectory(): Promise<string | null> {
    return openDesktopShelfDirectory();
  }

  addShelf(name: string, libRoot: string): Promise<DesktopShelfInfo | null> {
    return addDesktopShelf(name, libRoot);
  }
}
