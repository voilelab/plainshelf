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

function detectMobileRuntime(): boolean {
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

// Latched on first call rather than re-read per call. The runtime cannot change
// within a page lifetime, but the preview flag lives in the URL, and an in-app
// `router.push` (e.g. MobileConnectPage.onSave → '/books') drops the query —
// which would otherwise silently disengage every mobile guard for the rest of
// the session and leave the browser preview, and the e2e suite built on it,
// writable. Native Android was never affected: Capacitor.isNativePlatform()
// ignores the URL.
//
// Lazy rather than evaluated at module load so `window` is ready.
let mobileRuntime: boolean | undefined;

export function isMobileRuntime(): boolean {
  if (mobileRuntime === undefined) {
    mobileRuntime = detectMobileRuntime();
  }
  return mobileRuntime;
}
