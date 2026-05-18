const { app, BrowserWindow, dialog } = require('electron');
const path = require('node:path');
const { spawn } = require('node:child_process');
const readline = require('node:readline');

const SIDECAR_READY_TIMEOUT_MS = 15_000;

let mainWindow;
let sidecarProcess;
let sidecarInfo;

function sidecarBinaryName() {
  return process.platform === 'win32' ? 'plainshelf-gui-sidecar.exe' : 'plainshelf-gui-sidecar';
}

function candidateSidecarPaths() {
  const configured = process.env.PLAINSHELF_SIDECAR;
  const candidates = [];

  if (configured) {
    candidates.push(configured);
  }

  if (app.isPackaged) {
    candidates.push(path.join(process.resourcesPath, 'bin', sidecarBinaryName()));
    candidates.push(path.join(process.resourcesPath, sidecarBinaryName()));
  }

  candidates.push(path.resolve(__dirname, '..', '..', 'bin', sidecarBinaryName()));
  candidates.push(path.resolve(__dirname, '..', '..', 'cmd', 'plainshelf-gui-sidecar', sidecarBinaryName()));

  return candidates;
}

function findSidecarPath() {
  const fs = require('node:fs');
  for (const candidate of candidateSidecarPaths()) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  return candidateSidecarPaths()[0];
}

function startSidecar() {
  return new Promise((resolve, reject) => {
    const sidecarPath = findSidecarPath();
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
