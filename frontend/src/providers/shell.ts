import type { Router } from 'vue-router';

import type { BookshelfProvider } from './bookshelfProvider';
import type { ShelfInfo } from '@/api/shelves';
import type { ShelfPicker } from '@/types/shelfPicker';
import type { DeviceDocumentStorage } from '@/storage/deviceDocument';

/**
 * What a host shell contributes to the shared app.
 *
 * The shared code does not reach into a shell; a shell registers itself here
 * during bootstrap and the shared code asks. That direction is the point:
 * without it, `providers/index.ts` has to import every provider a shell might
 * pick, which puts the whole mobile and pCloud stack in the module graph of the
 * web build that `frontend/web.go` embeds and the desktop build serves.
 *
 * Kept to what a shell actually has to answer today. Add a member when a
 * concrete branch needs one, not in anticipation.
 */
export interface RuntimeShell {
  /**
   * The provider this shell reads its shelf through.
   *
   * Called lazily, on first use of the provider, so a shell may rely on
   * anything it set up during installation.
   */
  createProvider(): BookshelfProvider;

  /**
   * Where this shell keeps a device-local document — the reading history and
   * reading stats, which are per-device state and never sent to the shelf.
   *
   * Optional: a shell that is happy with the browser default does not implement
   * it, and the stores fall back to localStorage. `path` is the shell-agnostic
   * key each store already defines for itself.
   */
  createDeviceDocumentStorage?(path: string): DeviceDocumentStorage;

  /**
   * The one shelf this shell is pointed at, when its shelf list is device-local
   * rather than something a server enumerates.
   *
   * The mobile device keeps its own list — several servers and pCloud folders
   * side by side — of which exactly one is active, and the others are not
   * shelves *of this shelf's* server. So the app-wide list collapses to this.
   * Absent on a shell whose shelves come from the server.
   */
  activeShelfInfo?(): ShelfInfo | null;

  /**
   * The shelf dropdown this shell puts in the sidebar, when its shelves are not
   * the ones a server enumerates.
   *
   * The mobile device lists its own entries — several servers and pCloud
   * folders — and picking one restarts the app rather than swapping shelves in
   * place. Absent on a shell that is happy with the server-backed picker.
   */
  createShelfPicker?(): ShelfPicker;

  /**
   * Navigation guards this shell needs on the shared router.
   *
   * The mobile shell gates every route behind a usable shelf entry and refuses
   * the pages a read-only client cannot use. Installed by main.ts before
   * `app.use(router)`, which is what triggers the first navigation — a guard
   * registered after that would let the first route through ungated.
   */
  installRouterGuards?(router: Router): void;
}

let shell: RuntimeShell | null = null;

/**
 * Installs the host shell. Call once, during bootstrap, before anything can
 * ask for a provider — `main.ts` does this ahead of `createApp`.
 *
 * Passing null clears it, which is what a test needs between cases.
 */
export function registerShell(next: RuntimeShell | null): void {
  shell = next;
}

/** The installed shell, or null when running as a plain web or desktop client. */
export function getShell(): RuntimeShell | null {
  return shell;
}
