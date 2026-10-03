// A small demo-only bar to take the shelf out as a zip and put one back. The
// zip's layout is a lib_root (books/, trash/), so it unzips into a native shelf.
// Load after boot.js, on the app page.
(() => {
  const ZIP_URL = '/_demo/shelf.zip';

  function button(label, onClick) {
    const b = document.createElement('button');
    b.type = 'button';
    b.textContent = label;
    b.style.cssText = 'font:inherit;padding:4px 10px;border-radius:6px;border:1px solid #9aa4b2;background:#fff;color:#1f2937;cursor:pointer';
    b.addEventListener('click', onClick);
    return b;
  }

  async function exportShelf() {
    const res = await window.plainshelfRequest({ method: 'GET', url: ZIP_URL, headers: {} });
    if (res.status !== 200) {
      alert(`Export failed: ${new TextDecoder().decode(res.body)}`);
      return;
    }
    const url = URL.createObjectURL(new Blob([res.body], { type: 'application/zip' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `plainshelf-${new Date().toISOString().slice(0, 10)}.zip`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  function importShelf() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.zip,application/zip';
    input.addEventListener('change', async () => {
      const file = input.files?.[0];
      if (!file || !confirm(`Replace every book in this demo with "${file.name}"?`)) return;
      const body = new Uint8Array(await file.arrayBuffer());
      const res = await window.plainshelfRequest({
        method: 'PUT', url: ZIP_URL, headers: { 'Content-Type': 'application/zip' }, body,
      });
      if (res.status !== 204) {
        alert(`Import failed: ${new TextDecoder().decode(res.body)}`);
        return;
      }
      await window.plainshelfReloadAll();
    });
    input.click();
  }

  function mount() {
    const bar = document.createElement('div');
    bar.id = 'plainshelf-demo-bar';
    bar.setAttribute('role', 'toolbar');
    bar.setAttribute('aria-label', 'Demo shelf');
    bar.style.cssText = 'position:fixed;right:12px;bottom:12px;z-index:2147483647;display:flex;gap:6px;align-items:center;' +
      'padding:6px 8px;border-radius:8px;background:rgba(243,244,246,.95);box-shadow:0 1px 4px rgba(0,0,0,.2);font:13px system-ui,sans-serif;color:#1f2937';
    const label = document.createElement('span');
    label.textContent = 'Demo';
    label.style.fontWeight = '600';
    bar.append(label, button('Export shelf', exportShelf), button('Import shelf…', importShelf));
    document.body.append(bar);
  }

  if (document.body) mount();
  else document.addEventListener('DOMContentLoaded', mount);
})();
