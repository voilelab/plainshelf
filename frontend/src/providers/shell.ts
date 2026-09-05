import type { Router } from 'vue-router';

import type { BookshelfProvider } from './bookshelfProvider';
import type { ShelfInfo } from '@/api/shelves';
import type { ShelfPicker } from '@/types/shelfPicker';
import type { DeviceDocumentStorage } from '@/storage/deviceDocument';

/**
 * What a host shell contributes to the shared app. A shell registers itself
 * during bootstrap and the shared code asks; the direction is the point,
 * because otherwise `providers/index.ts` imports every provider a shell might
 * pick and the whole mobile and pCloud stack lands in the web bundle
 * `frontend/web.go` embeds.
 *
 * Add a member when a concrete branch needs one, not in anticipation.
 */
export interface RuntimeShell {
  /** Called lazily, so a shell may rely on anything it set up during install. */
  createProvider(): BookshelfProvider;

  /**
   * Where this shell keeps a device-local document — reading history and stats,
   * which are per-device state and never sent to the shelf. Optional: without
   * it the stores fall back to localStorage. `path` is the shell-agnostic key
   * each store already defines for itself.
   */
  createDeviceDocumentStorage?(path: string): DeviceDocumentStorage;

  /**
   * The one shelf this shell is pointed at, when its shelf list is device-local.
   * The mobile device keeps several servers and pCloud folders side by side, of
   * which exactly one is active and the others are not shelves *of this shelf's*
   * server, so the app-wide list collapses to this.
   */
  activeShelfInfo?(): ShelfInfo | null;

  /**
   * The shelf dropdown this shell puts in the sidebar. On mobile, picking one
   * restarts the app rather than swapping shelves in place. Absent on a shell
   * happy with the server-backed picker.
   */
  createShelfPicker?(): ShelfPicker;

  /**
   * Navigation guards this shell needs on the shared router. Installed by
   * main.ts before `app.use(router)`, which is what triggers the first
   * navigation — a guard registered after it would let the first route through
   * ungated.
   */
  installRouterGuards?(router: Router): void;
}

let shell: RuntimeShell | null = null;

/**
 * Call once, during bootstrap, before anything can ask for a provider —
 * `main.ts` does this ahead of `createApp`. Null clears it, for tests.
 */
export function registerShell(next: RuntimeShell | null): void {
  shell = next;
}

/** The installed shell, or null when running as a plain web or desktop client. */
export function getShell(): RuntimeShell | null {
  return shell;
}
