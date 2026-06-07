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

type CapacitorRuntime = {
  getPlatform?: () => string;
  isNativePlatform?: () => boolean;
};

export function isMobileRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const capacitor = (window as { Capacitor?: CapacitorRuntime }).Capacitor;
  if (!capacitor) {
    return false;
  }

  return capacitor.isNativePlatform?.() === true && capacitor.getPlatform?.() === 'android';
}
