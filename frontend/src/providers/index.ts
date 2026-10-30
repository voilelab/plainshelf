import { ApiError } from '@/api/client';
import type {
  BookshelfProvider,
  WritableBookshelfProvider
} from './bookshelfProvider';
import { isWailsRuntime } from './runtime';
import { getShell } from './shell';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import { WailsBookshelfProvider } from './wailsBookshelfProvider';

let provider: BookshelfProvider | null = null;

export function createBookshelfProvider(): BookshelfProvider {
  const shell = getShell();
  if (shell) {
    return shell.createProvider();
  }

  if (isWailsRuntime()) {
    return new WailsBookshelfProvider();
  }

  return new ServerBookshelfProvider();
}

export function getBookshelfProvider(): BookshelfProvider {
  if (!provider) {
    provider = createBookshelfProvider();
  }
  return provider;
}

const WRITES_UNAVAILABLE_MESSAGE = 'This client is read-only. Write operations are disabled.';

export function isWritableProvider(
  provider: BookshelfProvider
): provider is WritableBookshelfProvider {
  return provider.writable === true;
}

export function bookshelfWriter(): WritableBookshelfProvider {
  const active = getBookshelfProvider();
  if (!isWritableProvider(active)) {
    throw new ApiError(WRITES_UNAVAILABLE_MESSAGE, { status: 403 });
  }
  return active;
}

export type {
  BookshelfProvider,
  BookshelfReader,
  BookshelfWriter,
  WritableBookshelfProvider
} from './bookshelfProvider';
export { isMobileRuntime, isMobileShellPreview, isReaderRuntime, isWailsRuntime, isWebRuntime } from './runtime';
export type { RuntimeShell } from './shell';
export { getShell, registerShell } from './shell';
