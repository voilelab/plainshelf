import { Capacitor } from '@capacitor/core';
import { Browser } from '@capacitor/browser';
import { exportMobileBookContent } from '@/providers/mobileBookExport';
import { getBookshelfProvider } from '@/providers';
import { createBrowserPlatform, type PlatformCapabilities } from '@/providers/platformCapabilities';

export function createMobilePlatform(): PlatformCapabilities {
  return {
    coversMustBeFetchedAsBlob: true,
    openExternalUrl: Capacitor.isNativePlatform()
      ? async (url) => { await Browser.open({ url }); }
      : createBrowserPlatform().openExternalUrl,
    async getStorageEstimate() {
      if (typeof navigator === 'undefined' || !navigator.storage?.estimate) return { supported: false };
      try {
        const { usage, quota } = await navigator.storage.estimate();
        return { supported: true, usage, quota };
      } catch {
        return { supported: false };
      }
    },
    ...(Capacitor.isNativePlatform() ? {
      async saveBookContentToFile(id: string, name: string) {
        const blob = await getBookshelfProvider().downloadBookContent(id);
        return exportMobileBookContent(blob, name);
      }
    } : {})
  };
}
