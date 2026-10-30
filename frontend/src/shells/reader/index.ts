import { createBrowserPlatform } from '@/providers/platformCapabilities';
import { registerShell } from '@/providers/shell';
import { ServerBookshelfProvider } from '@/providers/serverBookshelfProvider';
import { activeReaderShelfInfo } from './shelfInfo';
import { installReaderRouterGuards } from './routerGuard';

export function installReaderShell(): void {
  registerShell({
    platform: createBrowserPlatform(),
    createProvider: () => new ServerBookshelfProvider(),
    activeShelfInfo: activeReaderShelfInfo,
    installRouterGuards: installReaderRouterGuards
  });
}
