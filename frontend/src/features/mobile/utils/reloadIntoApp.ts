// Capacitor Android 8.5.2 WebViewLocalServer serves index.html for extensionless paths with the default html5mode.
export function reloadIntoApp(path = '/books'): void {
  const url = new URL(window.location.href);
  const target = new URL(path, url.origin);
  url.pathname = target.pathname;
  target.searchParams.forEach((value, key) => url.searchParams.set(key, value));
  url.hash = '';
  window.location.replace(url.toString());
}
