// Writes one file into OPFS per message for opfs.js. A worker because
// createSyncAccessHandle exists in every engine with OPFS, while the
// main-thread createWritable only reached Safari in version 26.
// Message: { id, root, parts, data }, parts already encoded; replies { id, error? }.
self.onmessage = async ({ data: { id, root, parts, data } }) => {
  try {
    let dir = await (await navigator.storage.getDirectory()).getDirectoryHandle(root, { create: true });
    for (const part of parts.slice(0, -1)) dir = await dir.getDirectoryHandle(part, { create: true });
    const file = await dir.getFileHandle(parts[parts.length - 1], { create: true });
    const handle = await file.createSyncAccessHandle();
    try {
      // Older Safari returns promises from these; awaiting a plain value is harmless.
      await handle.truncate(0);
      await handle.write(data, { at: 0 });
      await handle.flush();
    } finally {
      await handle.close();
    }
    self.postMessage({ id });
  } catch (err) {
    self.postMessage({ id, error: String(err) });
  }
};
