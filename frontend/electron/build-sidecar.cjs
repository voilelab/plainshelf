const path = require('node:path');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');

const outputName = process.platform === 'win32' ? 'plainshelf-gui-sidecar.exe' : 'plainshelf-gui-sidecar';
const outputDir = path.resolve(__dirname);
const outputPath = path.join(outputDir, outputName);
const packagePath = path.resolve(__dirname, '..', '..', 'cmd', 'plainshelf-gui-sidecar');

fs.mkdirSync(outputDir, { recursive: true });

const result = spawnSync('go', ['build', '-o', outputPath, packagePath], {
  stdio: 'inherit'
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 0);
