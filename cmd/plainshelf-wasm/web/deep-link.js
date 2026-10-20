(() => {
  const base = '__BASE__';
  const { pathname, search, hash } = location;
  if (pathname.startsWith(base) && !search.includes('demo-route=')) {
    location.replace(`${base}?demo-route=${encodeURIComponent(pathname.slice(base.length) + search + hash)}`);
  }
})();
