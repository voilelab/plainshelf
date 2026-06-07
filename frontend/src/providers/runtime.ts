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

  // Placeholder for future Capacitor/Android runtime detection. Do not select it
  // until a mobile provider is implemented and dependencies are present.
  return false;
}
