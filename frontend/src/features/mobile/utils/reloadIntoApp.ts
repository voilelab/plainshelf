// Capacitor Android 8.5.2 WebViewLocalServer serves index.html for extensionless paths with the default html5mode.
export function reloadIntoApp(path = '/books'): void {
  const url = new URL(window.location.href);
  url.pathname = path;
  url.hash = '';
  window.location.replace(url.toString());
}
