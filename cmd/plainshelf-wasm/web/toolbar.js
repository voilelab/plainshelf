// A small demo-only bar to take the shelf out as a zip and put one back, with a
// note that the shelf lives only in browser storage and an offer to ask the
// browser to keep it. The zip's layout is a
// lib_root (books/, trash/), so it unzips into a native shelf.
// Load after boot.js, on the app page.
(() => {
  const ZIP_URL = '/_demo/shelf.zip';
  const MAX_IMPORT_BYTES = 512 << 20; // shelfzip's limit, checked before the file is read

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

  // The picker opens straight from the click: an await first could cost the
  // user activation browsers require for it.
  function importShelf() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.zip,application/zip';
    input.addEventListener('change', async () => {
      const file = input.files?.[0];
      if (!file) return;
      // An import reloads every tab, which only keeps the new shelf if it persists.
      if (!(await window.plainshelfDemo.hasStorage())) {
        alert('This browser cannot keep the demo shelf (no OPFS), so an imported shelf would be lost on reload.');
        return;
      }
      if (file.size > MAX_IMPORT_BYTES) {
        alert('Import failed: the zip is larger than 512 MiB.');
        return;
      }
      if (!confirm(`Replace every book in this demo with "${file.name}"?`)) return;
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

  const DISMISS_KEY = 'plainshelf-demo-persist-dismissed';
  const demo = window.plainshelfDemo;

  function dismissed() {
    try { return localStorage.getItem(DISMISS_KEY) === '1'; } catch { return false; }
  }

  function dismiss() {
    try { localStorage.setItem(DISMISS_KEY, '1'); } catch { /* the prompt just returns next visit */ }
  }

  // Inline display beats the hidden attribute, so the prompt toggles display.
  function show(el, visible) {
    el.style.display = visible ? 'flex' : 'none';
  }

  // Browser storage is not guaranteed, so the bar says where the shelf lives
  // and, until the browser agrees to keep it, offers to ask.
  async function refresh(note, prompt) {
    if (!(await demo.hasStorage())) {
      note.textContent = 'Not saved: this browser has no storage for the demo, so a reload starts over.';
      note.style.color = '#92400e';
      note.style.fontWeight = '600';
      show(prompt, false);
      return;
    }
    const kept = await demo.persisted();
    note.textContent = kept
      ? 'Kept in this browser, which has agreed not to clear it. Export to keep a copy elsewhere.'
      : 'Kept only in this browser, which may clear it. Export to keep a copy.';
    // A decline stays on screen until closed.
    if (kept || prompt.dataset.declined !== '1') show(prompt, !kept && demo.canPersist() && !dismissed());
  }

  function persistPrompt(onDone) {
    const prompt = document.createElement('div');
    prompt.id = 'plainshelf-demo-persist';
    prompt.style.cssText = 'flex-basis:100%;display:none;flex-wrap:wrap;gap:6px;align-items:center;font-size:12px';
    const text = document.createElement('span');
    text.style.flexBasis = '100%';
    text.textContent = 'Ask the browser to keep this shelf, so it is not cleared when space runs low?';
    const keep = button('Keep this shelf', async () => {
      keep.disabled = true;
      const granted = await demo.persist().catch(() => false);
      keep.disabled = false;
      if (granted) return onDone();
      prompt.dataset.declined = '1';
      text.textContent = 'The browser declined. Some browsers agree only for a bookmarked, installed or often visited site. You can ask again in Settings.';
      dismiss();
      keep.remove();
      later.textContent = 'OK';
    });
    const later = button('Not now', () => {
      dismiss();
      show(prompt, false);
    });
    prompt.append(text, keep, later);
    return prompt;
  }

  function mount() {
    const bar = document.createElement('div');
    bar.id = 'plainshelf-demo-bar';
    bar.setAttribute('role', 'toolbar');
    bar.setAttribute('aria-label', 'Demo shelf');
    bar.style.cssText = 'position:fixed;right:12px;bottom:12px;z-index:2147483647;display:flex;flex-wrap:wrap;gap:6px;align-items:center;' +
      'max-width:min(360px,calc(100vw - 24px));box-sizing:border-box;' +
      'padding:6px 8px;border-radius:8px;background:rgba(243,244,246,.95);box-shadow:0 1px 4px rgba(0,0,0,.2);font:13px system-ui,sans-serif;color:#1f2937';
    const label = document.createElement('span');
    label.textContent = 'Demo';
    label.style.fontWeight = '600';
    const note = document.createElement('span');
    note.id = 'plainshelf-demo-note';
    note.textContent = 'Kept only in this browser, which may clear it. Export to keep a copy.';
    note.style.cssText = 'flex-basis:100%;font-size:12px;color:#4b5563';
    const update = () => refresh(note, prompt).catch(() => {});
    const prompt = persistPrompt(update);
    bar.append(label, button('Export shelf', exportShelf), button('Import shelf…', importShelf), note, prompt);
    document.body.append(bar);
    addEventListener(demo.persistEvent, update);
    update();
  }

  if (document.body) mount();
  else document.addEventListener('DOMContentLoaded', mount);
})();
