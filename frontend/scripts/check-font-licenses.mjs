import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const fonts = [
  {
    packageName: '@fontsource-variable/noto-serif-tc',
    expectedVersion: '5.3.0',
    noticePath: 'public/licenses/noto-serif-tc-OFL-1.1.txt'
  },
  {
    packageName: '@fontsource-variable/noto-sans-tc',
    expectedVersion: '5.3.0',
    noticePath: 'public/licenses/noto-sans-tc-OFL-1.1.txt'
  }
];

for (const font of fonts) {
  const packageRoot = resolve('node_modules', font.packageName);
  const packageMetadata = JSON.parse(await readFile(resolve(packageRoot, 'package.json'), 'utf8'));
  const packageLicense = await readFile(resolve(packageRoot, 'LICENSE'), 'utf8');
  const publicNotice = await readFile(resolve(font.noticePath), 'utf8');

  if (packageMetadata.license !== 'OFL-1.1') {
    throw new Error(`${font.packageName} changed license to ${packageMetadata.license ?? 'unknown'}`);
  }

  if (packageMetadata.version !== font.expectedVersion) {
    throw new Error(`${font.packageName} must remain pinned to ${font.expectedVersion}`);
  }

  if (packageLicense !== publicNotice) {
    throw new Error(`${font.noticePath} does not exactly match ${font.packageName}/LICENSE`);
  }
}

console.log('Bundled font licenses and public notices are valid.');
