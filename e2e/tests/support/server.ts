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
const serverStartAttempts = 3;
const portTakenMarker = 'bind: address already in use';
// Node 22 fetch has no overall request deadline; AbortSignal.timeout bounds the health probe independently of transport timeouts.
const healthProbeTimeoutMs = 2_000;
const tempRootRemovalRetries = 4;

export type ServerEnv = {
  baseUrl: string;
  shelfDir: string;
  logs: string[];
  dispose: () => Promise<void>;
};

const portBandBase = 21_000;
const portBandSize = 100;

// Playwright 1.62.1 preserves TEST_PARALLEL_INDEX when replacing a worker (lib/runner/index.js and workerProcessEntry.js).
const workerParallelIndex = Number(process.env.TEST_PARALLEL_INDEX ?? 0);
let nextPortOffset = 0;

async function canBind(port: number): Promise<boolean> {
  const probe = net.createServer();
  const bound = await new Promise<boolean>((resolve) => {
    probe.once('error', () => resolve(false));
    probe.listen(port, '127.0.0.1', () => resolve(true));
  });
  if (!bound) {
    return false;
  }
  await new Promise<void>((resolve) => probe.close(() => resolve()));
  return true;
}

async function getWorkerBandPort(): Promise<number> {
  for (let tried = 0; tried < portBandSize; tried++) {
    const port = portBandBase + workerParallelIndex * portBandSize + (nextPortOffset % portBandSize);
    nextPortOffset = (nextPortOffset + 1) % portBandSize;
    if (await canBind(port)) {
      return port;
    }
  }

  throw new Error(
    `No free port in the E2E band ${portBandBase + workerParallelIndex * portBandSize}` +
      `-${portBandBase + (workerParallelIndex + 1) * portBandSize - 1} for worker ${workerParallelIndex}.`
  );
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

class PortTakenError extends Error {
  constructor(baseUrl: string) {
    super(`Another process holds the port of ${baseUrl}; retrying with a new one.`);
    this.name = 'PortTakenError';
  }
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
    let healthy = false;
    try {
      const response = await fetch(`${baseUrl}/health`, {
        signal: AbortSignal.timeout(healthProbeTimeoutMs)
      });
      healthy = response.ok && (await response.text()).trim() === '1';
    } catch {
    }

    if (logs.join('').includes(portTakenMarker)) {
      throw new PortTakenError(baseUrl);
    }
    if (healthy) {
      return;
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

async function removeTempRoot(tempRoot: string): Promise<void> {
  await fs.rm(tempRoot, {
    recursive: true,
    force: true,
    maxRetries: tempRootRemovalRetries,
    retryDelay: 100
  });
}

export async function startServer(): Promise<ServerEnv> {
  for (let attempt = 1; ; attempt++) {
    try {
      return await startServerOnce();
    } catch (error) {
      if (!(error instanceof PortTakenError) || attempt >= serverStartAttempts) {
        throw error;
      }
    }
  }
}

async function startServerOnce(): Promise<ServerEnv> {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'plainshelf-e2e-'));
  const shelfDir = path.join(tempRoot, 'shelf');
  const storeDir = path.join(tempRoot, 'store');
  const configPath = path.join(tempRoot, 'config.yaml');
  const logs: string[] = [];

  await fs.mkdir(shelfDir, { recursive: true });
  await fs.mkdir(storeDir, { recursive: true });

  const port = await getWorkerBandPort();
  const baseUrl = `http://127.0.0.1:${port}`;
  await fs.writeFile(configPath, buildConfigYAML(port, shelfDir, storeDir), 'utf8');

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
    await removeTempRoot(tempRoot).catch(() => undefined);
    throw error;
  }

  return {
    baseUrl,
    shelfDir,
    logs,
    dispose: async () => {
      await stopServer(server);
      await removeTempRoot(tempRoot);
    }
  };
}

export function useServer(): () => ServerEnv {
  let workerServer: ServerEnv | undefined;

  test.beforeAll(async () => {
    workerServer = await startServer();
  });

  test.afterAll(async () => {
    await workerServer?.dispose();
    workerServer = undefined;
  });

  return () => {
    if (!workerServer) {
      throw new Error('useServer() accessor read before beforeAll ran; call it at the spec top level.');
    }
    return workerServer;
  };
}
