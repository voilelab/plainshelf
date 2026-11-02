import { Capacitor } from '@capacitor/core';

let desktopPreview: boolean | undefined;

function hasDesktopPreviewFlag(): boolean {
  if (desktopPreview === undefined) {
    const params = new URLSearchParams(window.location.search);
    desktopPreview = params.get('desktop-shell-preview') === '1';
  }
  return desktopPreview;
}

export function isDesktopHost(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return Boolean((window as { __PLAINSHELF_DESKTOP__?: unknown }).__PLAINSHELF_DESKTOP__);
}

export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return isDesktopHost() || isReaderRuntime() || hasDesktopPreviewFlag();
}

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

export function isMobileShellPreview(): boolean {
  return hasMobilePreviewFlag();
}

function detectMobileRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  if (Capacitor.isNativePlatform()) {
    return true;
  }

  return hasMobilePreviewFlag();
}

let mobileRuntime: boolean | undefined;

export function isMobileRuntime(): boolean {
  if (mobileRuntime === undefined) {
    mobileRuntime = detectMobileRuntime();
  }
  return mobileRuntime;
}

export function isReaderRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return Boolean((window as { __PLAINSHELF_READER__?: unknown }).__PLAINSHELF_READER__);
}

export function isWebRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  return !isWailsRuntime() && !isMobileRuntime() && !isReaderRuntime();
}
