import { chromium } from '@playwright/test';
const browser = await chromium.launch({ executablePath: '/usr/local/share/chromium/chrome-linux/chrome', args: ['--no-sandbox'] });
const page = await browser.newPage({ viewport: { width: 1547, height: 900 } });
await page.goto('http://127.0.0.1:5173/');
await page.waitForTimeout(1500);
// find a book link
const links = await page.$$eval('a[href^="/books/"]', els => els.map(e => e.getAttribute('href')));
console.log('links sample:', links.slice(0,5));
if (links.length) {
  await page.goto('http://127.0.0.1:5173' + '/books/book-9');
  await page.waitForTimeout(1500);
  const info = await page.evaluate(() => {
    function rect(sel) {
      const el = document.querySelector(sel);
      if (!el) return null;
      const r = el.getBoundingClientRect();
      const cs = getComputedStyle(el);
      return { sel, left: r.left, right: r.right, width: r.width, bg: cs.backgroundColor, bgImage: cs.backgroundImage.slice(0,30) };
    }
    return {
      innerWidth: window.innerWidth,
      mainContent: rect('.main-content'),
      mainScroll: rect('.main-scroll'),
      pageArea: rect('.page-area'),
      detailShell: rect('.detail-shell'),
      detailContainer: rect('.detail-container'),
      detailPanel: rect('.detail-panel'),
      scrollbarWidth: document.querySelector('.main-scroll') ? document.querySelector('.main-scroll').offsetWidth - document.querySelector('.main-scroll').clientWidth : null,
    };
  });
  console.log(JSON.stringify(info, null, 2));
}
await browser.close();
