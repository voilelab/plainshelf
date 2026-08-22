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
  // A registered shell decides first: it is the thing that knows what this
  // host reads from, and asking it here is what keeps the mobile and pCloud
  // stack out of this module's import graph — and therefore out of the web
  // bundle that frontend/web.go embeds.
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
 * Mirrors how a read-only server answers a mutation (server/app.go), and how
 * PCloudBookshelfProvider refuses one, so a caller that surfaces the message
 * reads the same either way. Contains "read-only" deliberately: the mobile e2e
 * suite asserts on that phrase.
 */
const WRITES_UNAVAILABLE_MESSAGE = 'This client is read-only. Write operations are disabled.';

/**
 * Whether this provider implements the whole write surface.
 *
 * An intersection is not a union, so `provider.writable === true` does not
 * narrow on its own — the predicate is what does the narrowing, and
 * `implements BookshelfWriter` on the class is what makes it true.
 */
export function isWritableProvider(
  provider: BookshelfProvider
): provider is WritableBookshelfProvider {
  return provider.writable === true;
}

/**
 * The active provider, for a caller that is about to mutate the shelf.
 *
 * Every shelf write goes through here rather than through
 * getBookshelfProvider(), so "this call changes the library" is visible at the
 * call site and there is one place to refuse it. Reads — including the
 * device-local ones on mobile, such as saveReadProgress and the reading
 * history — keep using getBookshelfProvider().
 *
 * Throws rather than rejecting: every call site awaits inside a try, so a
 * synchronous throw is caught wherever this can currently be raised. Do not
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
