import { openDesktopExternalURL } from '../api/desktop';
import { isMobileRuntime, isWailsRuntime } from '../providers/runtime';

export async function openExternalURL(url: string): Promise<void> {
  if (isWailsRuntime()) {
    await openDesktopExternalURL(url);
    return;
  }

  if (typeof window === 'undefined') {
    return;
  }

  const target = isMobileRuntime() ? '_system' : '_blank';
  const opened = window.open(url, target, 'noopener,noreferrer');
  if (!opened) {
    window.location.href = url;
  }
}
