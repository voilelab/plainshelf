import { importDesktopBooksFromLocalPaths, openDesktopBookFiles } from '../api/desktop';
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
}
