import { Capacitor } from '@capacitor/core';

// Latched on first read for the same reason as the mobile flag below: an
// in-app `router.push` drops the query string, which would otherwise turn the
// desktop preview off mid-session and leave the browser preview — and the e2e
// suite built on it — indistinguishable from a plain web build.
//
// Only the URL flag is latched. The three real-runtime signals stay live per
// call, so a Wails binding that lands after the first read (the first read is
// initAppZoom() in main.ts) still switches desktop mode on, instead of being
// frozen out by a latch taken too early.
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

// Desktop-browser preview of the mobile shell, mirroring the Wails
// `?desktop-shell-preview=1` escape hatch above.
//
// Latched on first read: the flag lives in the URL, and an in-app `router.push`
// (e.g. MobileConnectPage.onSave → '/books') drops the query — which would
// otherwise silently disengage every mobile guard for the rest of the session
// and leave the browser preview, and the e2e suite built on it, writable.
// Native Android was never affected: Capacitor.isNativePlatform() ignores the
// URL.
//
// One latch serves both readers below, so the runtime and the preview cannot
// disagree about the same query string.
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
 * Whether the mobile shell is being previewed in an ordinary browser rather
 * than running natively.
 *
 * True only for `?mobile-shell-preview=1`; a native Capacitor shell is a mobile
 * runtime but never a preview. That distinction is what keeps browser-only
 * affordances — the e2e provider hooks in main.ts — out of shipped app builds.
 */
export function isMobileShellPreview(): boolean {
  return hasMobilePreviewFlag();
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

  return hasMobilePreviewFlag();
}

// Latched on first call rather than re-read per call, so the runtime cannot
// flip mid-session. The URL half of the decision is latched by
// hasMobilePreviewFlag() above — see the reasoning there; this second latch
// covers the Capacitor half, which is stable for a page lifetime anyway.
//
// Lazy rather than evaluated at module load so `window` is ready.
let mobileRuntime: boolean | undefined;

export function isMobileRuntime(): boolean {
  if (mobileRuntime === undefined) {
    mobileRuntime = detectMobileRuntime();
  }
  return mobileRuntime;
}

/**
 * Whether this is the standalone reader app rather than the desktop client or a
 * browser.
 *
 * The reader injects its boot config into index.html before the app's scripts
 * run (see shells/reader/bootConfig), so this is answerable during bootstrap —
 * which is what lets main.ts install the reader shell before the first
 * navigation, instead of after a request has come back.
 *
 * Not latched: the flag is written into the page itself, and the app reloads
 * the window when a book is opened, so it cannot change under a running app.
 */
export function isReaderRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return Boolean((window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__);
}
