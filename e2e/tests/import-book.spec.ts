import { expect, test, type Page } from '@playwright/test';
import { once } from 'node:events';
import { promises as fs } from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { spawn, type ChildProcess } from 'node:child_process';

const repoRoot = path.resolve(__dirname, '..', '..');
const fixturePath = path.resolve(__dirname, '..', 'fixtures', 'hello.txt');
const tinyPngBase64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9s3FoXcAAAAASUVORK5CYII=';
const serverStartupTimeoutMs = 30_000;
const serverShutdownTimeoutMs = 10_000;

type ServerEnv = {
  baseUrl: string;
  logs: string[];
  dispose: () => Promise<void>;
};

async function getFreePort(): Promise<number> {
  const server = net.createServer();
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => resolve());
  });

  const address = server.address();
  if (!address || typeof address === 'string') {
    await new Promise<void>((resolve, reject) => server.close((err) => (err ? reject(err) : resolve())));
    throw new Error('Failed to determine a free TCP port for the E2E server.');
  }

  await new Promise<void>((resolve, reject) => server.close((err) => (err ? reject(err) : resolve())));
  return address.port;
}

function buildConfigYAML(port: number, shelfDir: string, storeDir: string): string {
  const baseUrl = `http://127.0.0.1:${port}`;
  return [
    'logger:',
    '  level: error',
    '  format: text',
    '  log_file:',
    '    type: stderr',
    'server_conf:',
    `  addr: "127.0.0.1:${port}"`,
    '  read_timeout: 60s',
    '  write_timeout: 60s',
    'app_conf:',
    '  logger:',
    '    level: error',
    '    format: text',
    '    log_file:',
    '      type: stderr',
    '  shelf:',
    '    logger:',
    '      level: error',
    '      format: text',
    '      log_file:',
    '        type: stderr',
    `    lib_root: ${JSON.stringify(shelfDir)}`,
    `  store_path: ${JSON.stringify(storeDir)}`,
    '  cover_to_jpg: false',
    '  read_history_limit: 10',
    '  security:',
    '    mode: "local_token"',
    '    protect_read: false',
    '    token_header: "X-PlainShelf-Token"',
    '    allow_missing_origin_with_token: true',
    '    allowed_origins:',
    `      - ${JSON.stringify(baseUrl)}`,
    ''
  ].join('\n');
}

async function waitForServer(baseUrl: string, server: ChildProcess, logs: string[]): Promise<void> {
  const deadline = Date.now() + serverStartupTimeoutMs;

  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(`Server exited early with code ${server.exitCode}.\n${logs.join('')}`);
    }
    if (server.signalCode) {
      throw new Error(`Server exited early with signal ${server.signalCode}.\n${logs.join('')}`);
    }

    try {
      const response = await fetch(`${baseUrl}/health`);
      if (response.ok && (await response.text()).trim() === '1') {
        return;
      }
    } catch {
      // Server is still starting.
    }

    await new Promise((resolve) => setTimeout(resolve, 250));
  }

  throw new Error(`Timed out waiting for ${baseUrl}/health.\n${logs.join('')}`);
}

function signalServer(server: ChildProcess, signal: NodeJS.Signals): void {
  const pid = server.pid;
  if (!pid) {
    server.kill(signal);
    return;
  }

  if (process.platform === 'win32') {
    server.kill(signal);
    return;
  }

  try {
    process.kill(-pid, signal);
  } catch (error) {
    const err = error as NodeJS.ErrnoException;
    if (err.code === 'ESRCH' || err.code === 'EINVAL' || err.code === 'ENOSYS') {
      server.kill(signal);
      return;
    }
    throw error;
  }
}

async function stopServer(server: ChildProcess): Promise<void> {
  if (server.exitCode !== null || server.signalCode) {
    return;
  }

  const exitPromise = once(server, 'exit');
  signalServer(server, 'SIGTERM');

  const timedOut = await Promise.race([
    exitPromise.then(() => false),
    new Promise<boolean>((resolve) => setTimeout(() => resolve(true), serverShutdownTimeoutMs))
  ]);

  if (timedOut) {
    signalServer(server, 'SIGKILL');
    await exitPromise;
  }
}

async function startServer(): Promise<ServerEnv> {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'plainshelf-e2e-'));
  const shelfDir = path.join(tempRoot, 'shelf');
  const storeDir = path.join(tempRoot, 'store');
  const configPath = path.join(tempRoot, 'config.yaml');
  const logs: string[] = [];

  await fs.mkdir(shelfDir, { recursive: true });
  await fs.mkdir(storeDir, { recursive: true });

  const port = await getFreePort();
  const baseUrl = `http://127.0.0.1:${port}`;
  await fs.writeFile(configPath, buildConfigYAML(port, shelfDir, storeDir), 'utf8');

  const server = spawn('go', ['run', './cmd/plainshelf-srv/main.go', '-conf', configPath], {
    cwd: repoRoot,
    env: process.env,
    detached: true,
    stdio: ['ignore', 'pipe', 'pipe']
  });

  server.stdout?.on('data', (chunk: Buffer | string) => {
    logs.push(String(chunk));
  });
  server.stderr?.on('data', (chunk: Buffer | string) => {
    logs.push(String(chunk));
  });

  try {
    await waitForServer(baseUrl, server, logs);
  } catch (error) {
    await stopServer(server).catch(() => undefined);
    await fs.rm(tempRoot, { recursive: true, force: true });
    throw error;
  }

  return {
    baseUrl,
    logs,
    dispose: async () => {
      await stopServer(server);
      await fs.rm(tempRoot, { recursive: true, force: true });
    }
  };
}

async function importHelloBook(page: Page): Promise<void> {
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expect(page.getByText('No books yet.')).toBeVisible();

  await page.getByRole('button', { name: /^Import/ }).click();
  await page.getByRole('button', { name: 'Import from files' }).click();

  const dialog = page.getByRole('dialog', { name: 'Import Book' });
  await expect(dialog).toBeVisible();
  await dialog.locator('input[type="file"]').setInputFiles(fixturePath);
  await expect(dialog.getByText('hello.txt')).toBeVisible();

  const importButton = dialog.getByRole('button', { name: 'Import', exact: true });
  await importButton.click();

  await expect(page.getByText('1 books')).toBeVisible();
  await expect(importButton).toBeDisabled();
  await expect(page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
}

async function createCoverDataTransfer(page: Page) {
  return await page.evaluateHandle((bytes) => {
    const dataTransfer = new DataTransfer();
    const binary = Uint8Array.from(atob(bytes), (char) => char.charCodeAt(0));
    dataTransfer.items.add(new File([binary], 'cover.png', { type: 'image/png' }));
    return dataTransfer;
  }, tinyPngBase64);
}

test('should import a txt book from the UI and render it in the reader', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await bookTitle.click();

    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await expect(page.getByRole('button', { name: 'Read' })).toBeVisible();
    await page.getByRole('button', { name: 'Read' }).click();

    await expect(page).toHaveURL(/\/reader\/[^/]+$/);
    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();
    await expect(page.getByText('This text came from a real uploaded TXT file.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should update a book cover from drag and drop on the detail page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);

    const coverTarget = page.locator('.cover-drop-target');
    await expect(coverTarget).toBeVisible();

    const dataTransfer = await createCoverDataTransfer(page);
    try {
      await coverTarget.dispatchEvent('dragenter', { dataTransfer });
      await expect(page.getByText('Drop image to update cover')).toBeVisible();
      await coverTarget.dispatchEvent('dragover', { dataTransfer });
      await coverTarget.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    const confirmDialog = page.getByRole('dialog', { name: 'Update book cover?' });
    await expect(confirmDialog).toBeVisible();
    await expect(confirmDialog.getByText('Do you want to update the book cover?')).toBeVisible();
    await confirmDialog.getByRole('button', { name: 'Update cover' }).click();

    await expect(confirmDialog).not.toBeVisible();
    await expect(page.getByRole('button', { name: 'Remove' })).toBeEnabled();
    await expect(page.locator('img.detail-cover')).toHaveAttribute(
      'src',
      /\/api\/shelves\/default_shelf\/books\/[^/]+\/cover(?:\?t=\d+)?$/
    );
  } finally {
    await server.dispose();
  }
});
