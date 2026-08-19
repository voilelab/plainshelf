/**
 * Fails when shared frontend code imports a mobile-only module.
 *
 * The mobile stack — its provider, its on-device caches, the pCloud client, the
 * shelf configuration and its Capacitor plugins — reaches the bundle through
 * `src/shells/mobile` and nothing else. That is what keeps it out of the web
 * build `frontend/web.go` embeds and the desktop build serves; a single stray
 * import in shared code silently puts all of it back, and nothing else in the
 * toolchain notices.
 *
 * Only *static* imports are refused. A dynamic `import()` is the sanctioned way
 * to reach this code — it is what `main.ts` uses for the shell itself, what the
 * router uses for the mobile pages, and what puts the module in its own chunk
 * rather than the eagerly loaded graph. Refusing those too would force a second
 * route table into the shell, which is worse than the branch it removes.
 *
 * Deliberately a plain script rather than a lint plugin: the repository has no
 * ESLint, and this rule is small enough that adding one — plus a dependency for
 * `check-licenses` to vet — would cost more than it explains.
 */
import { readdir, readFile } from 'node:fs/promises';
import { join, relative } from 'node:path';

const SRC = 'src';

/** Modules only the mobile shell may reach. Matched against the import specifier. */
const MOBILE_ONLY = [
  /^@capacitor\//,
  /^@\/api\/pcloud\//,
  /^@\/features\/mobile\//,
  /^@\/providers\/(mobileConfig|mobileBookshelfProvider|mobileBookCache|mobileCacheFs|mobileCoverCache|filesystemMobileBookCache|filesystemMobileCoverCache|pcloudBookshelfProvider|shelfSnapshotStore|secureStorage|cacheScope)$/,
  /^\.\/(mobileConfig|mobileBookshelfProvider|mobileBookCache|mobileCacheFs|mobileCoverCache|filesystemMobileBookCache|filesystemMobileCoverCache|pcloudBookshelfProvider|shelfSnapshotStore|secureStorage|cacheScope)$/
];

/** Files allowed to reach them: the shell itself, the mobile feature area, and the mobile modules. */
const MOBILE_SIDE = [
  /^src\/shells\//,
  /^src\/features\/mobile\//,
  /^src\/api\/pcloud\//,
  /^src\/providers\/(mobileConfig|mobileBookshelfProvider|mobileBookCache|mobileCacheFs|mobileCoverCache|filesystemMobileBookCache|filesystemMobileCoverCache|pcloudBookshelfProvider|shelfSnapshotStore|secureStorage|cacheScope)\.ts$/
];

/**
 * providers/runtime.ts is the one shared module that may touch Capacitor: it
 * answers "are we in a native shell?", which main.ts must ask before it can
 * decide whether to load the shell at all. It imports only @capacitor/core.
 */
const CAPACITOR_EXEMPT = new Map([['src/providers/runtime.ts', new Set(['@capacitor/core'])]]);

const STATIC_IMPORT_PATTERN = /(?:^|\n)\s*(?:import|export)[^;\n]*?from\s*['"]([^'"]+)['"]/g;

async function* walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      yield* walk(path);
    } else if (/\.(ts|vue)$/.test(entry.name) && !/\.test\.ts$/.test(entry.name)) {
      yield path;
    }
  }
}

const violations = [];

for await (const path of walk(SRC)) {
  const file = relative('.', path).replaceAll('\\', '/');
  if (MOBILE_SIDE.some((pattern) => pattern.test(file))) {
    continue;
  }

  const source = await readFile(path, 'utf8');
  const exempt = CAPACITOR_EXEMPT.get(file) ?? new Set();

  STATIC_IMPORT_PATTERN.lastIndex = 0;
  for (const match of source.matchAll(STATIC_IMPORT_PATTERN)) {
    const specifier = match[1];
    if (exempt.has(specifier)) {
      continue;
    }
    if (MOBILE_ONLY.some((mobile) => mobile.test(specifier))) {
      violations.push(`${file} imports ${specifier}`);
    }
  }
}

if (violations.length > 0) {
  console.error('Shared code must not import mobile-only modules:\n');
  for (const violation of violations) {
    console.error(`  ${violation}`);
  }
  console.error(
    '\nReach the mobile stack through src/shells/mobile instead — add a RuntimeShell\n' +
      'member (src/providers/shell.ts) and let the shell register it.'
  );
  process.exit(1);
}

console.log(`Module boundaries OK (${MOBILE_ONLY.length} rules).`);
