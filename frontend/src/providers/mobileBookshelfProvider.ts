import type { DownloadState, PaginatedBooks } from '../types/book';
import { ServerBookshelfProvider } from './serverBookshelfProvider';

const notImplemented = (operation: string): Error =>
  new Error(`MobileBookshelfProvider.${operation} is not implemented yet`);

export class MobileBookshelfProvider extends ServerBookshelfProvider {
  // Placeholder for a future Capacitor/Android provider. It should combine:
  // TODO: remote API client for online source-of-truth reads.
  // TODO: local metadata DB for download state, versions, timestamps, progress, and bookmarks.
  // TODO: local source content cache for downloaded book/source content.
  // TODO: offline downloaded-only listing and open/read behavior.

  listBooks(page?: number, pageSize?: number, search?: string): Promise<PaginatedBooks> {
    // Until local cache plumbing exists, reuse the server provider as the remote
    // API client for online/source-of-truth library listing.
    return super.listBooks(page, pageSize, search);
  }

  async getDownloadState(_bookId: string): Promise<DownloadState> {
    throw notImplemented('getDownloadState');
  }

  async downloadBook(_bookId: string): Promise<void> {
    throw notImplemented('downloadBook');
  }

  async removeDownload(_bookId: string): Promise<void> {
    throw notImplemented('removeDownload');
  }
}
