// Static server for a site from build.sh, with an SPA fallback to index.html.
// Usage: node serve.mjs <site dir> [port]
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const types = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.wasm': 'application/wasm',
  '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon',
  '.woff2': 'font/woff2', '.webmanifest': 'application/manifest+json',
};

export async function serveStatic(siteDir, port = 0) {
  const root = path.resolve(siteDir);
  const srv = http.createServer((req, res) => {
    let file = path.join(root, decodeURIComponent(new URL(req.url, 'http://x').pathname));
    if (!file.startsWith(root + path.sep)) file = root; // no escaping the site dir
    if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(root, 'index.html');
    res.writeHead(200, { 'content-type': types[path.extname(file)] ?? 'application/octet-stream' });
    fs.createReadStream(file).pipe(res);
  });
  await new Promise((ok) => srv.listen(port, '127.0.0.1', ok));
  return srv;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [dir, port = '5180'] = process.argv.slice(2);
  if (!dir) {
    console.error('usage: node serve.mjs <site dir> [port]');
    process.exit(2);
  }
  const srv = await serveStatic(dir, Number(port));
  console.log(`PlainShelf wasm demo: http://127.0.0.1:${srv.address().port}/ (data lives in the tab; a reload starts empty)`);
}
