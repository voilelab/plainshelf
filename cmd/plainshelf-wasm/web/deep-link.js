// Inlined into the host's 404 page by build.sh. A link or reload into one of the
// demo's routes is a path the static host does not have; send it to the demo,
// which restores the route (boot.js) before the router reads it.
(() => {
  const base = '__BASE__';
  const { pathname, search, hash } = location;
  if (pathname.startsWith(base) && !search.includes('demo-route=')) {
    location.replace(`${base}?demo-route=${encodeURIComponent(pathname.slice(base.length) + search + hash)}`);
  }
})();
