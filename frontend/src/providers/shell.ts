import type { PlatformCapabilities } from './platformCapabilities';
import type { Router } from 'vue-router';

import type { BookshelfProvider } from './bookshelfProvider';
import type { ShelfInfo } from '@/api/shelves';
import type { ShelfPicker } from '@/types/shelfPicker';
import type { DeviceDocumentStorage } from '@/storage/deviceDocument';

export interface RuntimeShell {
  platform?: PlatformCapabilities;
  createProvider(): BookshelfProvider;

  createDeviceDocumentStorage?(path: string): DeviceDocumentStorage;

  activeShelfInfo?(): ShelfInfo | null;
  loadActiveShelfInfo?(shelf: ShelfInfo): Promise<ShelfInfo>;

  createShelfPicker?(): ShelfPicker;

  installRouterGuards?(router: Router): void;
}

let shell: RuntimeShell | null = null;

export function registerShell(next: RuntimeShell | null): void {
  shell = next;
}

export function getShell(): RuntimeShell | null {
  return shell;
}
