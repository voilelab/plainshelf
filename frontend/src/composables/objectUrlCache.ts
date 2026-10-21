interface ObjectUrlEntry {
  url?: string;
  promise?: Promise<string>;
  refCount: number;
}

const objectUrlCache = new Map<string, ObjectUrlEntry>();

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
    const capturedCacheEntry = entry;
    entry.promise = fetchBlob()
      .then((blob) => {
        const url = URL.createObjectURL(blob);
        const currentCacheEntry = objectUrlCache.get(key);

        if (currentCacheEntry !== capturedCacheEntry) {
          URL.revokeObjectURL(url);
          return url;
        }

        currentCacheEntry.url = url;
        return url;
      })
      .catch((err) => {
        objectUrlCache.delete(key);
        throw err;
      });
  }

  return entry.promise;
}

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
