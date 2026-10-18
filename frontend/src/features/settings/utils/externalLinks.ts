import { getPlatformCapabilities } from '@/providers/platformCapabilities';

export async function openExternalURL(url: string): Promise<void> {
  await getPlatformCapabilities().openExternalUrl?.(url);
}
