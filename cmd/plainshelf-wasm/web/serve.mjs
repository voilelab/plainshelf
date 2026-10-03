// Static server for a site from build.sh. An unknown path gets index.html, the
// SPA fallback, or, when the site has a 404.html, that page with status 404, as
// GitHub Pages does.
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
    const { pathname, search } = new URL(req.url, 'http://x');
    let file = path.join(root, decodeURIComponent(pathname));
    if (file !== root && !file.startsWith(root + path.sep)) { // no escaping the site dir
      res.writeHead(404).end();
      return;
    }
    if (fs.existsSync(file) && fs.statSync(file).isDirectory()) {
      if (!pathname.endsWith('/')) {
        res.writeHead(301, { location: pathname + '/' + search }).end();
        return;
      }
      file = path.join(file, 'index.html');
    }
    let status = 200;
    if (!fs.existsSync(file)) {
      const notFound = path.join(root, '404.html');
      status = fs.existsSync(notFound) ? 404 : 200;
      file = status === 404 ? notFound : path.join(root, 'index.html');
    }
    res.writeHead(status, { 'content-type': types[path.extname(file)] ?? 'application/octet-stream' });
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
  console.log(`PlainShelf wasm demo: http://127.0.0.1:${srv.address().port}/ (the shelf is kept in this browser's OPFS)`);
}
