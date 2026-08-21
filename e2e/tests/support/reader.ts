import { once } from 'node:events';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { spawn, type ChildProcess } from 'node:child_process';

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const readerStartupTimeoutMs = 60_000;
const readerShutdownTimeoutMs = 10_000;

export type ReaderEnv = {
  baseUrl: string;
  logs: string[];
  dispose: () => Promise<void>;
};

/**
 * The address line `cmd/plainshelf-read` prints on startup.
 *
 * The binary binds 127.0.0.1:0 and lets the kernel choose the port — that is
 * what lets several copies read several shelves at once — so what it prints is
 * the only way to find it. Parsing it here also makes the line itself part of
 * what the suite covers: a user has nothing else to go on either.
 */
const addressLine = /Open (http:\/\/127\.0\.0\.1:\d+)\//;

function findBaseUrl(logs: string[]): string | null {
  const match = addressLine.exec(logs.join(''));
  return match ? match[1] : null;
}

async function waitForReader(reader: ChildProcess, logs: string[]): Promise<string> {
  const deadline = Date.now() + readerStartupTimeoutMs;

  while (Date.now() < deadline) {
    if (reader.exitCode !== null) {
      throw new Error(`plainshelf-read exited early with code ${reader.exitCode}.\n${logs.join('')}`);
    }
    if (reader.signalCode) {
      throw new Error(`plainshelf-read exited early with signal ${reader.signalCode}.\n${logs.join('')}`);
    }

    const baseUrl = findBaseUrl(logs);
    if (baseUrl) {
      try {
        const response = await fetch(`${baseUrl}/health`);
        if (response.ok && (await response.text()).trim() === '1') {
          return baseUrl;
        }
      } catch {
        // Listening but not answering yet.
      }
    }

    await new Promise((resolve) => setTimeout(resolve, 250));
  }

  throw new Error(`Timed out waiting for plainshelf-read to print its address.\n${logs.join('')}`);
}

async function stopReader(reader: ChildProcess): Promise<void> {
  if (reader.exitCode !== null || reader.signalCode) {
    return;
  }

  const exitPromise = once(reader, 'exit');
  reader.kill('SIGTERM');

  const timedOut = await Promise.race([
    exitPromise.then(() => false),
    new Promise<boolean>((resolve) => setTimeout(() => resolve(true), readerShutdownTimeoutMs))
  ]);

  if (timedOut) {
    reader.kill('SIGKILL');
    await exitPromise;
  }
}

/**
 * Starts the standalone reader against an existing shelf folder.
 *
 * `-no-browser` is not optional here: without it the binary hands the address
 * to the desktop environment, which on a CI machine is either nothing or a
 * browser nobody asked for.
 *
 * The shelf folder is not removed on dispose — it belongs to whoever seeded it,
 * and a reader that deleted it would defeat the point of the spec that asserts
 * the folder came through untouched.
 */
export async function startReader(shelfDir: string): Promise<ReaderEnv> {
  const logs: string[] = [];

  const reader = spawn('go', ['run', './cmd/plainshelf-read', '-no-browser', shelfDir], {
    cwd: repoRoot,
    env: process.env,
    stdio: ['ignore', 'pipe', 'pipe']
  });

  reader.stdout?.on('data', (chunk: Buffer | string) => {
    logs.push(String(chunk));
  });
  reader.stderr?.on('data', (chunk: Buffer | string) => {
    logs.push(String(chunk));
  });

  try {
    const baseUrl = await waitForReader(reader, logs);
    return {
      baseUrl,
      logs,
      dispose: () => stopReader(reader)
    };
  } catch (error) {
    await stopReader(reader).catch(() => undefined);
    throw error;
  }
}

/**
 * Every path under root with its size and modification time, so two snapshots
 * can show that a run of the reader changed nothing.
 */
export async function snapshotTree(root: string): Promise<Record<string, string>> {
  const states: Record<string, string> = {};

  async function walk(dir: string): Promise<void> {
    for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      const rel = path.relative(root, full);
      if (entry.isDirectory()) {
        states[rel] = 'dir';
        await walk(full);
        continue;
      }
      const info = await fs.stat(full);
      states[rel] = `${info.size}@${info.mtimeMs}`;
    }
  }

  await walk(root);
  return states;
}
