/**
 * Restarts the app at the library, discarding everything the previous shelf
 * left in memory.
 *
 * Every shelf switch needs this, not just the ones involving pCloud.
 * PCloudBookshelfProvider captures its client and shelf path at construction
 * and the provider is a singleton built once at bootstrap; the shelf, folder and
 * server-mode stores are module-level too. A plain `router.push` would leave
 * all of it pointing at the shelf the user just left.
 *
 * Loading a deep path directly is safe in the native shell: Capacitor serves
 * index.html for any path whose last segment has no extension (html5mode, on by
 * default — WebViewLocalServer.java). The query string is kept so the browser
 * preview stays in mobile mode, which a reload would otherwise lose along with
 * the latched runtime.
 */
export function reloadIntoApp(path = '/books'): void {
  const url = new URL(window.location.href);
  url.pathname = path;
  url.hash = '';
  window.location.replace(url.toString());
}
