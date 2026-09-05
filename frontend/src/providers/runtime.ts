import { Capacitor } from '@capacitor/core';

// Latched for the same reason as the mobile flag below: an in-app `router.push`
// drops the query string, which would turn the preview off mid-session.
//
// Only the URL flag is latched. The three real-runtime signals stay live per
// call, so a Wails binding that lands after the first read still switches
// desktop mode on rather than being frozen out by a latch taken too early.
let desktopPreview: boolean | undefined;

function hasDesktopPreviewFlag(): boolean {
  if (desktopPreview === undefined) {
    const params = new URLSearchParams(window.location.search);
    desktopPreview = params.get('desktop-shell-preview') === '1';
  }
  return desktopPreview;
}

export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const hasWailsBinding = Boolean((window as { go?: unknown }).go);

  return (
    hasWailsBinding ||
    window.location.protocol === 'wails:' ||
    window.location.host.endsWith('.wails.localhost') ||
    hasDesktopPreviewFlag()
  );
}

// Desktop-browser preview of the mobile shell, mirroring the Wails escape
// hatch above.
//
// Latched on first read: an in-app `router.push` (MobileConnectPage.onSave →
// '/books') drops the query, which would silently disengage every mobile guard
// for the rest of the session and leave the browser preview writable. One latch
// serves both readers below, so they cannot disagree about the same query.
let mobilePreview: boolean | undefined;

function hasMobilePreviewFlag(): boolean {
  if (mobilePreview === undefined) {
    if (typeof window === 'undefined') {
      return false;
    }
    const params = new URLSearchParams(window.location.search);
    mobilePreview = params.get('mobile-shell-preview') === '1';
  }
  return mobilePreview;
}

/**
 * True only for `?mobile-shell-preview=1`; a native Capacitor shell is a mobile
 * runtime but never a preview. That distinction keeps browser-only affordances —
 * the e2e provider hooks in main.ts — out of shipped app builds.
 */
export function isMobileShellPreview(): boolean {
  return hasMobilePreviewFlag();
}

function detectMobileRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  // Capacitor's own check rather than UA sniffing: true only inside a native
  // shell, so an ordinary mobile browser or PWA is not mistaken for one.
  if (Capacitor.isNativePlatform()) {
    return true;
  }

  return hasMobilePreviewFlag();
}

// Latched so the runtime cannot flip mid-session, and lazy so `window` is ready.
// The URL half is latched by hasMobilePreviewFlag() above.
let mobileRuntime: boolean | undefined;

export function isMobileRuntime(): boolean {
  if (mobileRuntime === undefined) {
    mobileRuntime = detectMobileRuntime();
  }
  return mobileRuntime;
}

/**
 * The reader injects its boot config into index.html before the app's scripts
 * run (see shells/reader/bootConfig), so this is answerable during bootstrap —
 * which lets main.ts install the reader shell before the first navigation.
 *
 * Not latched: the flag is written into the page itself, so it cannot change
 * under a running app.
 */
export function isReaderRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return Boolean((window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__);
}

/**
 * The plain web build `frontend/web.go` embeds. Defined as the negation of the
 * three real shells, so a shell added later is excluded here the moment its own
 * predicate reports it instead of silently counting as web.
 */
export function isWebRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return !isWailsRuntime() && !isMobileRuntime() && !isReaderRuntime();
}
