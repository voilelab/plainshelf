interface ObjectUrlEntry {
  url?: string;
  promise?: Promise<string>;
  refCount: number;
}

// Module-level so object URLs are shared and ref-counted across every consumer
// that ends up showing the same resource (e.g. the same book appearing in both
// the shelf grid and a "recently read" list, or the same illustration used
// twice in one chapter), not just within a single component instance.
const objectUrlCache = new Map<string, ObjectUrlEntry>();

/**
 * Returns a `blob:` object URL for `key`, fetching it through `fetchBlob` on
 * first use and reusing it afterwards.
 *
 * Every call takes a reference; each one must be matched by exactly one
 * `releaseObjectUrl(key)` or the underlying blob is never revoked. Callers pick
 * the key, so it has to identify the bytes: two resources that can differ must
 * not share one.
 */
export function acquireObjectUrl(key: string, fetchBlob: () => Promise<Blob>): Promise<string> {
  let entry = objectUrlCache.get(key);
  if (!entry) {
    entry = { refCount: 0 };
    objectUrlCache.set(key, entry);
  }
  entry.refCount += 1;

  if (entry.url) {
    return Promise.resolve(entry.url);
  }

  if (!entry.promise) {
    const pending = entry;
    entry.promise = fetchBlob()
      .then((blob) => {
        const url = URL.createObjectURL(blob);
        const current = objectUrlCache.get(key);

        // The last holder can let go while the fetch is still in flight -- a
        // reader leaving the chapter before a large illustration arrives. The
        // entry is gone by then (or has been replaced by a fresh one), so
        // nothing would ever release this URL: revoke it here instead of
        // leaking it. Identity, not presence, is the test, so a re-acquire
        // that started its own fetch is not clobbered.
        if (current !== pending) {
          URL.revokeObjectURL(url);
          return url;
        }

        current.url = url;
        return url;
      })
      .catch((err) => {
        // Do not cache a failed fetch: drop the entry so a later attempt
        // (next mount, or the server becoming reachable again) can retry
        // instead of being stuck replaying the same rejection.
        objectUrlCache.delete(key);
        throw err;
      });
  }

  return entry.promise;
}

/** Drops one reference taken by `acquireObjectUrl`, revoking at the last one. */
export function releaseObjectUrl(key: string): void {
  const entry = objectUrlCache.get(key);
  if (!entry) {
    return;
  }
  entry.refCount -= 1;
  if (entry.refCount > 0) {
    return;
  }
  if (entry.url) {
    URL.revokeObjectURL(entry.url);
  }
  objectUrlCache.delete(key);
}
