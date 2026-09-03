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
/**
 * How many times a server start may pick a fresh port after losing a race for
 * the previous one. See {@link portTakenMarker}.
 */
const serverStartAttempts = 3;
/**
 * The server logs this and then *keeps running* without a listener, so a lost
 * port race looks like a healthy process that never answers /health. Matching
 * the line turns a 30-second timeout into an immediate retry.
 */
const portTakenMarker = 'bind: address already in use';
/**
 * Per-probe cap on the /health request. Without it a probe can block forever:
 * when the port belongs to a process that accepts connections and never
 * answers, `fetch` has no timeout of its own, so the poll loop stops ticking
 * and `serverStartupTimeoutMs` is never reached — the run fails much later with
 * Playwright's bare "beforeAll hook timeout" and none of the server's output.
 */
const healthProbeTimeoutMs = 2_000;
/** How many times a busy temp shelf may be re-tried before teardown fails. */
const tempRootRemovalAttempts = 5;

export type ServerEnv = {
  baseUrl: string;
  shelfDir: string;
  logs: string[];
  dispose: () => Promise<void>;
};

/**
 * Base of the port bands handed out below. Above the privileged range and
 * below Linux's ephemeral range (32768+), so a band never overlaps a port the
 * kernel might hand to something else on the machine.
 */
const portBandBase = 21_000;
/** Ports per worker. Bands are disjoint, so two workers cannot collide. */
const portBandSize = 100;

/**
 * Playwright numbers its workers 0..workers-1 and reuses the number when a
 * worker is replaced, so it is a stable band index rather than a run counter.
 * Absent (a plain `node` caller, or a spec run outside the runner) means one
 * server at a time, which band 0 serves.
 */
const parallelIndex = Number(process.env.TEST_PARALLEL_INDEX ?? 0);
/** Rotates within the band so a restart rarely lands on the port just freed. */
let nextPortOffset = 0;

/** Resolves true when nothing holds `port`, false when the bind is refused. */
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

/**
 * Picks a port from this worker's own band.
 *
 * Asking the kernel for port 0 would be shorter, but it makes the port a
 * lottery across the whole machine: two workers can be handed the same number,
 * and the loser then finds a *healthy* /health answer on it — the other
 * worker's server, over the other worker's shelf. A per-worker band removes
 * the collision instead of detecting it. Within one worker, starts are
 * sequential and a live server still holds its port, so the bind probe is
 * enough to keep two of them apart.
 */
async function getFreePort(): Promise<number> {
  for (let tried = 0; tried < portBandSize; tried++) {
    const port = portBandBase + parallelIndex * portBandSize + (nextPortOffset % portBandSize);
    nextPortOffset = (nextPortOffset + 1) % portBandSize;
    if (await canBind(port)) {
      return port;
    }
  }

  throw new Error(
    `No free port in the E2E band ${portBandBase + parallelIndex * portBandSize}` +
      `-${portBandBase + (parallelIndex + 1) * portBandSize - 1} for worker ${parallelIndex}.`
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

/** Distinguishes a lost port race, which is worth retrying, from a real failure. */
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
      // Server is still starting.
    }

    // After the probe, not only before it: a healthy answer on a port we lost
    // comes from *another worker's* server, and taking it would quietly hand
    // two workers the same shelf. Our own process reports the lost bind here.
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

/**
 * Deletes a temp shelf, tolerating a directory that is still busy.
 *
 * The server is gone by the time this runs, but its last writes are not
 * necessarily visible to `rm` yet, and the whole-suite failure recorded in
 * `.claude/rules/50-lessons.md` was exactly that: `ENOTEMPTY … rmdir
 * '<tmp>/shelf/app'`, landing on a different spec each run. Parallel workers
 * multiply the chances, so retry briefly instead of failing a green spec in
 * teardown.
 */
async function removeTempRoot(tempRoot: string): Promise<void> {
  // `fs.rm`'s own maxRetries only covers EBUSY/EMFILE/ENFILE/ENOTEMPTY/EPERM
  // on Windows, so the backoff has to be ours.
  for (let attempt = 1; ; attempt++) {
    try {
      await fs.rm(tempRoot, { recursive: true, force: true });
      return;
    } catch (error) {
      const code = (error as NodeJS.ErrnoException).code;
      const retryable = code === 'ENOTEMPTY' || code === 'EBUSY' || code === 'EPERM';
      if (!retryable || attempt >= tempRootRemovalAttempts) {
        throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, attempt * 100));
    }
  }
}

/**
 * Starts one server on its own port, over its own temp shelf and store.
 *
 * Every call is independent, which is what lets `playwright.config.ts` run
 * fully parallel: no state is shared between two of these, not even the
 * directory names.
 */
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
