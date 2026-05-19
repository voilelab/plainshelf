const { app, BrowserWindow, dialog } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const { spawn } = require('node:child_process');
const readline = require('node:readline');

const SIDECAR_READY_TIMEOUT_MS = 15_000;

let mainWindow;
let sidecarProcess;
let sidecarInfo;

function sidecarBinaryName() {
  return process.platform === 'win32' ? 'plainshelf-gui-sidecar.exe' : 'plainshelf-gui-sidecar';
}

function resolveSidecarPath() {
  const configuredPath = process.env.PLAINSHELF_SIDECAR;
  if (configuredPath !== undefined) {
    const trimmed = configuredPath.trim();
    if (trimmed.length === 0) {
      throw new Error('PLAINSHELF_SIDECAR is set but empty. Refusing to continue.');
    }

    const explicitPath = path.resolve(trimmed);
    if (!fs.existsSync(explicitPath)) {
      throw new Error(`Configured sidecar not found: ${explicitPath}`);
    }
    return explicitPath;
  }

  const localPath = path.join(__dirname, sidecarBinaryName());
  if (!fs.existsSync(localPath)) {
    throw new Error(`Sidecar not found next to Electron main script: ${localPath}`);
  }
  return localPath;
}

function startSidecar() {
  return new Promise((resolve, reject) => {
    const sidecarPath = resolveSidecarPath();
    const args = [];

    if (process.env.PLAINSHELF_PROFILE) {
      args.push('-profile', process.env.PLAINSHELF_PROFILE);
    }
    if (process.env.PLAINSHELF_SIDECAR_ADDR) {
      args.push('-addr', process.env.PLAINSHELF_SIDECAR_ADDR);
    }

    const child = spawn(sidecarPath, args, {
      stdio: ['pipe', 'pipe', 'pipe'],
      windowsHide: true
    });
    sidecarProcess = child;

    const timeout = setTimeout(() => {
      reject(new Error(`Timed out waiting for PlainShelf sidecar readiness from ${sidecarPath}`));
      stopSidecar();
    }, SIDECAR_READY_TIMEOUT_MS);

    child.once('error', (error) => {
      clearTimeout(timeout);
      reject(new Error(`Failed to start PlainShelf sidecar at ${sidecarPath}: ${error.message}`));
    });

    child.stderr.on('data', (chunk) => {
      process.stderr.write(`[sidecar] ${chunk}`);
    });

    const rl = readline.createInterface({ input: child.stdout });
    rl.on('line', (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch (error) {
        process.stderr.write(`[sidecar stdout] ${line}\n`);
        return;
      }

      if (event.type === 'ready') {
        clearTimeout(timeout);
        sidecarInfo = event;
        resolve(event);
        return;
      }

      if (event.type === 'error') {
        clearTimeout(timeout);
        reject(new Error(event.error || 'PlainShelf sidecar failed to start.'));
      }
    });

    child.once('exit', (code, signal) => {
      if (!sidecarInfo) {
        clearTimeout(timeout);
        reject(new Error(`PlainShelf sidecar exited before ready (code=${code}, signal=${signal}).`));
      }
      sidecarProcess = undefined;
    });
  });
}

function stopSidecar() {
  if (!sidecarProcess) {
    return;
  }

  const child = sidecarProcess;
  sidecarProcess = undefined;

  if (child.stdin.writable) {
    child.stdin.write('shutdown\n');
    child.stdin.end();
  }

  const killTimer = setTimeout(() => {
    if (!child.killed) {
      child.kill('SIGTERM');
    }
  }, 5_000);

  child.once('exit', () => clearTimeout(killTimer));
}

function createWindow(info) {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 840,
    minWidth: 960,
    minHeight: 640,
    title: 'PlainShelf',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      additionalArguments: [
        `--plainshelf-api-token=${info.token}`,
        `--plainshelf-token-header=${info.token_header}`,
        `--plainshelf-base-url=${info.base_url}`,
        `--plainshelf-profile-dir=${info.profile_dir}`
      ]
    }
  });

  mainWindow.loadURL(info.base_url);
}

app.whenReady().then(async () => {
  try {
    const info = await startSidecar();
    createWindow(info);
  } catch (error) {
    dialog.showErrorBox('PlainShelf failed to start', error instanceof Error ? error.message : String(error));
    app.quit();
  }
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0 && sidecarInfo) {
    createWindow(sidecarInfo);
  }
});

app.on('before-quit', () => {
  stopSidecar();
});
