import { registerShell } from '@/providers/shell';
import { ServerBookshelfProvider } from '@/providers/serverBookshelfProvider';
import { activeReaderShelfInfo } from './shelfInfo';
import { installReaderRouterGuards } from './routerGuard';

/**
 * Brings up the standalone reader shell.
 *
 * The reader app serves the same read-only HTTP surface the server does, so the
 * provider is the ordinary server one: what makes this a reader is the shelf it
 * reports and the routes it allows, both of which live here rather than in the
 * shared app.
 *
 * This module is the only entry point into the reader shell, so everything it
 * pulls in reaches the bundle through here and through nothing else.
 */
export function installReaderShell(): void {
  registerShell({
    createProvider: () => new ServerBookshelfProvider(),
    activeShelfInfo: activeReaderShelfInfo,
    installRouterGuards: installReaderRouterGuards
  });
}
