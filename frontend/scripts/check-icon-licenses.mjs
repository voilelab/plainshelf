/**
 * Guards the icon set the bundle inlines.
 *
 * unplugin-icons copies Tabler's SVG paths into the JavaScript output at build
 * time, so the shipped app is a redistribution of the icon set even though
 * `@iconify-json/tabler` is a devDependency `license-checker --production`
 * never inspects. MIT then requires the copyright notice to travel with it,
 * which `public/licenses/tabler-icons-MIT.txt` is: a plain file the build
 * serves at /licenses/.
 *
 * Pinning the exact version is what makes that notice checkable — an icon set
 * can relicense between releases, and a caret range would pull the new terms in
 * silently.
 */
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const iconSets = [
  {
    packageName: '@iconify-json/tabler',
    expectedVersion: '1.2.38',
    expectedSpdx: 'MIT',
    noticePath: 'public/licenses/tabler-icons-MIT.txt'
  }
];

/** The clause MIT requires a redistribution to carry, normalized to one line. */
const MIT_RETENTION_CLAUSE =
  'The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.';

for (const iconSet of iconSets) {
  const packageRoot = resolve('node_modules', iconSet.packageName);
  const packageMetadata = JSON.parse(await readFile(resolve(packageRoot, 'package.json'), 'utf8'));
  const setInfo = JSON.parse(await readFile(resolve(packageRoot, 'info.json'), 'utf8'));
  const notice = await readFile(iconSet.noticePath, 'utf8');

  if (packageMetadata.version !== iconSet.expectedVersion) {
    throw new Error(`${iconSet.packageName} must remain pinned to ${iconSet.expectedVersion}`);
  }

  if (setInfo.license?.spdx !== iconSet.expectedSpdx) {
    throw new Error(`${iconSet.packageName} changed license to ${setInfo.license?.spdx ?? 'unknown'}`);
  }

  if (!/^Copyright \(c\) .+$/m.test(notice)) {
    throw new Error(`${iconSet.noticePath} has no copyright line`);
  }

  if (!notice.replace(/\s+/g, ' ').includes(MIT_RETENTION_CLAUSE)) {
    throw new Error(`${iconSet.noticePath} is not the ${iconSet.expectedSpdx} notice it claims to be`);
  }
}

console.log('Bundled icon set licenses and public notices are valid.');
