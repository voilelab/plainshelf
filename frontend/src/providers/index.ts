import type { BookshelfProvider } from './bookshelfProvider';
import { isMobileRuntime, isWailsRuntime } from './runtime';
import { MobileBookshelfProvider } from './mobileBookshelfProvider';
import { IndexedDbMobileBookCache } from './indexedDbMobileBookCache';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import { WailsBookshelfProvider } from './wailsBookshelfProvider';

let provider: BookshelfProvider | null = null;

export function createBookshelfProvider(): BookshelfProvider {
  if (isWailsRuntime()) {
    return new WailsBookshelfProvider();
  }

  if (isMobileRuntime()) {
    // Persist downloads and reading progress across app restarts.
    return new MobileBookshelfProvider(new ServerBookshelfProvider(), new IndexedDbMobileBookCache());
  }

  return new ServerBookshelfProvider();
}

export function getBookshelfProvider(): BookshelfProvider {
  if (!provider) {
    provider = createBookshelfProvider();
  }
  return provider;
}

export type { BookshelfProvider } from './bookshelfProvider';
export { isMobileRuntime, isWailsRuntime } from './runtime';
export type { CachedBookManifest, MobileBookCache } from './mobileBookCache';
export { InMemoryMobileBookCache } from './mobileBookCache';
export { IndexedDbMobileBookCache } from './indexedDbMobileBookCache';
