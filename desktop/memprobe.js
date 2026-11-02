window.__memprobe = (() => {
  const shelfID = 'default_shelf';
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  let bookIDs = [];

  function router() {
    return document.querySelector('#app').__vue_app__.config.globalProperties.$router;
  }

  async function loadBooks() {
    const resp = await fetch(`/api/shelves/${shelfID}/books`);
    const books = await resp.json();
    books.sort((a, b) => a.meta.title.localeCompare(b.meta.title));
    bookIDs = books.map((b) => b.meta.id);
    return bookIDs.length;
  }

  function scrollables() {
    return [...document.querySelectorAll('*')].filter((el) => {
      const style = getComputedStyle(el);
      return /(auto|scroll)/.test(style.overflowY) && el.scrollHeight > el.clientHeight + 10;
    });
  }

  async function scrollThrough(steps, delayMs) {
    for (let i = 0; i < steps; i++) {
      for (const el of scrollables()) {
        el.scrollTop += el.clientHeight;
      }
      await sleep(delayMs);
    }
  }

  async function library() {
    await router().push('/books');
    await sleep(3000);
    await scrollThrough(40, 300);
  }

  async function readLarge(sections) {
    await router().push(`/reader/${bookIDs[0]}`);
    await sleep(4000);
    for (let i = 0; i < sections; i++) {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
      await sleep(400);
      await scrollThrough(3, 100);
    }
  }

  async function hopBooks(count) {
    for (let i = 1; i <= count && i < bookIDs.length; i++) {
      await router().push(`/reader/${bookIDs[i]}`);
      await sleep(600);
    }
  }

  async function cycle() {
    if (bookIDs.length === 0) {
      await loadBooks();
    }
    await library();
    await readLarge(60);
    await hopBooks(40);
    await router().push('/home');
    await sleep(2000);
    return location.pathname;
  }

  function mark(phase) {
    return window.go.main.DesktopApp.MemprobeMark(phase);
  }

  async function run(idleMs, cycles, settleMs) {
    while (!window.go?.main?.DesktopApp?.MemprobeMark || !document.querySelector('#app')?.__vue_app__) {
      await sleep(200);
    }
    await mark('idle');
    await sleep(idleMs);
    await loadBooks();
    for (let i = 1; i <= cycles; i++) {
      await mark(`cycle ${i}`);
      await cycle();
      await mark(`cycle ${i} settle`);
      await sleep(settleMs);
    }
    await mark('final idle');
    await sleep(idleMs);
    await mark('done');
  }

  return { loadBooks, cycle, run };
})();
