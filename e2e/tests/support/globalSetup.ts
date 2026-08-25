import { spawnSync } from 'node:child_process';
import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '..', '..', '..');

/**
 * Environment variable through which globalSetup hands the compiled server
 * binary to every worker's `startServer()`. When it is set, the server is
 * spawned directly from the executable; when it is absent (e.g. a spec run
 * without this globalSetup), `startServer()` falls back to `go run`.
 */
export const serverBinaryEnvVar = 'PLAINSHELF_E2E_SERVER_BIN';

/**
 * Compiles the server once, before any spec runs, so the 28 spec files spawn a
 * ready-made executable instead of paying for a `go run` cold compile on every
 * server start. The binary path is published through {@link serverBinaryEnvVar}
 * for `startServer()` to pick up.
 */
export default async function globalSetup(): Promise<void> {
  // Honour a binary the caller already built (e.g. the justfile's `test-e2e`, or
  // a developer iterating on one spec) so the suite compiles the server at most
  // once regardless of how it was launched.
  const preBuilt = process.env[serverBinaryEnvVar];
  if (preBuilt) {
    // eslint-disable-next-line no-console
    console.log(`Reusing E2E server binary: ${preBuilt}`);
    return;
  }

  const binDir = await fs.mkdtemp(path.join(os.tmpdir(), 'plainshelf-e2e-bin-'));
  const binaryName = process.platform === 'win32' ? 'plainshelf-srv.exe' : 'plainshelf-srv';
  const binaryPath = path.join(binDir, binaryName);

  const started = Date.now();
  const result = spawnSync('go', ['build', '-o', binaryPath, './cmd/plainshelf-srv/main.go'], {
    cwd: repoRoot,
    stdio: ['ignore', 'inherit', 'inherit']
  });

  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`go build for the E2E server failed with exit code ${result.status}.`);
  }

  process.env[serverBinaryEnvVar] = binaryPath;
  // eslint-disable-next-line no-console
  console.log(`Built E2E server binary in ${((Date.now() - started) / 1000).toFixed(1)}s: ${binaryPath}`);
}
