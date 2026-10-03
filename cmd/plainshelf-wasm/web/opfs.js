// Persists memfs to the Origin Private File System: load() restores the tree
// before Go starts, then changes are written back shortly after they happen.
// Load after memfs.js. Without OPFS support the demo stays in memory.
(() => {
  const ROOT = 'plainshelf-demo';
  const DEBOUNCE_MS = 200;

  const supported = typeof navigator !== 'undefined' && navigator.storage?.getDirectory && typeof Worker !== 'undefined';
  const writerUrl = new URL('opfs-writer.js', document.currentScript.src);

  // Names are stored percent-encoded: Chromium 141 resolves a non-ASCII OPFS
  // name to the parent directory, which flattened every CJK-titled book.
  const toOPFS = (name) => encodeURIComponent(name);
  const fromOPFS = (name) => {
    try {
      return decodeURIComponent(name);
    } catch {
      return name;
    }
  };

  let rootHandle;
  let writer;
  const writes = new Map(); // id -> { resolve, reject }
  let nextWrite = 0;
  let timer;
  let queue = Promise.resolve(); // writes run one batch at a time, in order

  async function dirAt(path, create) {
    let h = rootHandle;
    for (const part of path.split('/').filter(Boolean)) h = await h.getDirectoryHandle(toOPFS(part), { create });
    return h;
  }

  async function restore(h, path) {
    for await (const [name, child] of h.entries()) {
      const p = `${path}/${fromOPFS(name)}`;
      if (child.kind === 'directory') {
        memfs.restoreDir(p);
        await restore(child, p);
      } else {
        const file = await child.getFile();
        memfs.restoreFile(p, new Uint8Array(await file.arrayBuffer()), file.lastModified);
      }
    }
  }

  // A dirty directory drops OPFS entries memfs no longer has; a dirty file is rewritten.
  async function write({ path, type, data }) {
    if (type === 'dir') {
      const h = await dirAt(path, true);
      const stale = [];
      for await (const [name, child] of h.entries()) {
        const kind = memfs.kindOf(`${path === '/' ? '' : path}/${fromOPFS(name)}`);
        if (kind !== (child.kind === 'directory' ? 'dir' : 'file')) stale.push(name);
      }
      for (const name of stale) await h.removeEntry(name, { recursive: true });
      return;
    }
    const parts = path.split('/').filter(Boolean).map(toOPFS);
    await new Promise((resolve, reject) => {
      const id = nextWrite++;
      writes.set(id, { resolve, reject });
      writer.postMessage({ id, root: ROOT, parts, data }, [data.buffer]);
    });
  }

  // A failed entry stays dirty and is retried with the next change.
  async function drain() {
    for (const entry of memfs.takeDirty()) {
      try {
        await write(entry);
      } catch (err) {
        console.error('plainshelf: could not persist', entry.path, err);
        memfs.markDirty(entry.path);
      }
    }
  }

  function flush() {
    clearTimeout(timer);
    timer = undefined;
    if (rootHandle) queue = queue.then(drain);
    return queue;
  }

  window.plainshelfStorage = {
    async load() {
      if (!supported) {
        console.warn('plainshelf: OPFS or workers are unavailable; the demo will not survive a reload');
        return;
      }
      rootHandle = await (await navigator.storage.getDirectory()).getDirectoryHandle(ROOT, { create: true });
      writer = new Worker(writerUrl);
      writer.onmessage = ({ data: { id, error } }) => {
        const pending = writes.get(id);
        writes.delete(id);
        if (error) pending?.reject(new Error(error));
        else pending?.resolve();
      };
      await restore(rootHandle, '');
      memfs.takeDirty(); // restored nodes are already on disk
      window.plainshelfStorage.persistent = true;
      // At most DEBOUNCE_MS after the first change, however busy the shelf is.
      memfs.onChange(() => {
        timer ??= setTimeout(flush, DEBOUNCE_MS);
      });
      addEventListener('pagehide', flush);
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'hidden') flush();
      });
    },
    flush,
  };
})();
