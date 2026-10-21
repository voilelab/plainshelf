import { spawnSync } from 'node:child_process';
import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '..', '..', '..');

export const serverBinaryEnvVar = 'PLAINSHELF_E2E_SERVER_BIN';

export default async function globalSetup(): Promise<(() => Promise<void>) | void> {
  const suppliedServerBinary = process.env[serverBinaryEnvVar];
  if (suppliedServerBinary) {
    // eslint-disable-next-line no-console
    console.log(`Reusing E2E server binary: ${suppliedServerBinary}`);
    return;
  }

  const binDir = await fs.mkdtemp(path.join(os.tmpdir(), 'plainshelf-e2e-bin-'));
  const binaryName = process.platform === 'win32' ? 'plainshelf-srv.exe' : 'plainshelf-srv';
  const binaryPath = path.join(binDir, binaryName);

  const buildStartedAt = Date.now();
  const result = spawnSync('go', ['build', '-o', binaryPath, './cmd/plainshelf-srv/main.go'], {
    cwd: repoRoot,
    stdio: ['ignore', 'inherit', 'inherit']
  });

  if (result.error || result.status !== 0) {
    await fs.rm(binDir, { recursive: true, force: true });
    throw result.error ?? new Error(`go build for the E2E server failed with exit code ${result.status}.`);
  }

  process.env[serverBinaryEnvVar] = binaryPath;
  // eslint-disable-next-line no-console
  console.log(`Built E2E server binary in ${((Date.now() - buildStartedAt) / 1000).toFixed(1)}s: ${binaryPath}`);

  return async () => {
    await fs.rm(binDir, { recursive: true, force: true });
  };
}
