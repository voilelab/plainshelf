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
  // A registered shell decides first: asking it here is what keeps the mobile
  // and pCloud stack out of this module's import graph, and therefore out of
  // the web bundle frontend/web.go embeds.
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

/**
 * Mirrors how a read-only server answers a mutation (server/app.go) and how
 * PCloudBookshelfProvider refuses one, so the message reads the same either way.
 * The phrase "read-only" is asserted on by the mobile e2e suite.
 */
const WRITES_UNAVAILABLE_MESSAGE = 'This client is read-only. Write operations are disabled.';

/**
 * An intersection is not a union, so `provider.writable === true` does not
 * narrow on its own: this predicate is what narrows, and `implements
 * BookshelfWriter` on the class is what makes it true.
 */
export function isWritableProvider(
  provider: BookshelfProvider
): provider is WritableBookshelfProvider {
  return provider.writable === true;
}

/**
 * Every shelf write goes through here rather than getBookshelfProvider(), so
 * "this call changes the library" is visible at the call site and there is one
 * place to refuse it. Reads — including the device-local ones on mobile — keep
 * using getBookshelfProvider().
 *
 * Throws rather than rejecting: every call site awaits inside a try. Do not
 * introduce `bookshelfWriter().x(…).catch(…)` in a non-async context.
 */
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
