import { once } from 'node:events';
import { promises as fs } from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { spawn, type ChildProcess } from 'node:child_process';
import { test } from '@playwright/test';
import { serverBinaryEnvVar } from './globalSetup';

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const serverStartupTimeoutMs = 30_000;
const serverShutdownTimeoutMs = 10_000;

export type ServerEnv = {
  baseUrl: string;
  shelfDir: string;
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
    '  shelves:',
    '    - id: default_shelf',
    '      name: Default Shelf',
    '      logger:',
    '        level: error',
    '        format: text',
    '        log_file:',
    '          type: stderr',
    `      lib_root: ${JSON.stringify(shelfDir)}`,
    `  store_path: ${JSON.stringify(storeDir)}`,
    '  cover_to_jpg: false',
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

export async function startServer(): Promise<ServerEnv> {
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

  // globalSetup compiles the server once and publishes its path here, so a
  // server start is just a process spawn. Without it (e.g. a spec run that
  // bypasses globalSetup), fall back to `go run` so the helper stays usable.
  const serverBinary = process.env[serverBinaryEnvVar];
  const command = serverBinary ?? 'go';
  const args = serverBinary
    ? ['-conf', configPath]
    : ['run', './cmd/plainshelf-srv/main.go', '-conf', configPath];

  const server = spawn(command, args, {
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
    shelfDir,
    logs,
    dispose: async () => {
      await stopServer(server);
      await fs.rm(tempRoot, { recursive: true, force: true });
    }
  };
}

/**
 * Registers one server for the whole spec file: `beforeAll` starts it and
 * `afterAll` disposes of it, so every test in the file shares the same
 * pre-built server (and its temp shelf/store) instead of paying for a fresh
 * boot each time. Call it once at the top level of a spec file and read the
 * environment inside each test through the returned accessor:
 *
 * ```ts
 * const getServer = useServer();
 * test('...', async ({ page }) => {
 *   const { baseUrl } = getServer();
 * });
 * ```
 *
 * The shared shelf persists across the file's tests, so each test must use its
 * own book and folder names rather than assuming an empty shelf. A spec that
 * genuinely needs a pristine server per test should keep calling
 * {@link startServer} directly and say why in a comment.
 */
export function useServer(): () => ServerEnv {
  let env: ServerEnv | undefined;

  test.beforeAll(async () => {
    env = await startServer();
  });

  test.afterAll(async () => {
    await env?.dispose();
    env = undefined;
  });

  return () => {
    if (!env) {
      throw new Error('useServer() accessor read before beforeAll ran; call it at the spec top level.');
    }
    return env;
  };
}
