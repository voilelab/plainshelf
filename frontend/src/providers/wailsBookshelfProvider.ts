import { addDesktopShelf, importDesktopBooksFromLocalPaths, openDesktopBookFiles, openDesktopShelfDirectory } from '../api/desktop';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import type { DesktopImportBookResult, DesktopShelfInfo } from './bookshelfProvider';

export class WailsBookshelfProvider extends ServerBookshelfProvider {
  openShelfDirectory(): Promise<string | null> {
    return openDesktopShelfDirectory();
  }

  addShelf(name: string, libRoot: string): Promise<DesktopShelfInfo | null> {
    return addDesktopShelf(name, libRoot);
  }

  openLocalBookFiles(): Promise<string[] | null> {
    return openDesktopBookFiles();
  }

  importBooksFromLocalPaths(
    localPaths: string[],
    layerPath: string
  ): Promise<DesktopImportBookResult[] | null> {
    return importDesktopBooksFromLocalPaths(localPaths, layerPath);
  }
}
