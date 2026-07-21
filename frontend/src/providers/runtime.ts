import { Capacitor } from '@capacitor/core';

export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const params = new URLSearchParams(window.location.search);
  const hasWailsBinding = Boolean((window as { go?: unknown }).go);

  return (
    hasWailsBinding ||
    window.location.protocol === 'wails:' ||
    window.location.host.endsWith('.wails.localhost') ||
    params.get('desktop-shell-preview') === '1'
  );
}

export function isMobileRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  // Prefer Capacitor's own platform check over UA sniffing: it is true only
  // inside a native iOS/Android shell, so an ordinary mobile browser (or PWA)
  // is not mistaken for the native runtime.
  if (Capacitor.isNativePlatform()) {
    return true;
  }

  // Desktop-browser preview of the mobile shell, mirroring the Wails
  // `?desktop-shell-preview=1` escape hatch above.
  const params = new URLSearchParams(window.location.search);
  return params.get('mobile-shell-preview') === '1';
}
