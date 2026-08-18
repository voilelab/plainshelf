import type { BookshelfProvider } from './bookshelfProvider';
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
