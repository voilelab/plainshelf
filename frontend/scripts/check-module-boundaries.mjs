/**
 * Fails when frontend code reaches across a module boundary it must not cross.
 *
 * The mobile stack — its provider, its on-device caches, the pCloud client, the
 * shelf configuration and its Capacitor plugins — reaches the bundle through
 * `src/shells/mobile` and nothing else. That is what keeps it out of the web
 * build `frontend/web.go` embeds and the desktop build serves; a single stray
 * import in shared code silently puts all of it back, and nothing else in the
 * toolchain notices.
 *
 * A feature directory is private to itself for a quieter reason: shared code
 * that happens to live under one is invisible as shared code. Nothing marks it
 * as having other callers, so it keeps the feature's assumptions in its name and
 * its allowlists, and the next caller inherits them. Only the reader is checked
 * today, because only the reader had grown that way.
 *
 * Only *static* imports are refused. A dynamic `import()` is the sanctioned way
 * to reach this code — it is what `main.ts` uses for the shell itself, what the
 * router uses for the mobile pages, and what puts the module in its own chunk
 * rather than the eagerly loaded graph. Refusing those too would force a second
 * route table into the shell, which is worse than the branch it removes.
 *
 * Deliberately a plain script rather than a lint plugin: the repository has no
 * ESLint, and these rules are small enough that adding one — plus a dependency
 * for `check-licenses` to vet — would cost more than they explain.
 */
import { readdir, readFile } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { pathToFileURL } from 'node:url';

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

/**
 * The reader feature is private to itself. Its Markdown rendering and its
 * sanitizer were shared code living behind a feature path, and the next caller
 * that needs them would have deepened the mistake rather than moved them out.
 */
const READER_PRIVATE = [/^@\/features\/reader\//];

/** Files allowed to reach them: the reader feature itself. */
const READER_SIDE = [/^src\/features\/reader\//];

/**
 * A static `import`/`export … from '…'` statement, which the repository writes
 * across several lines as often as one. The statement may therefore span
 * newlines, and only a `;` ends it — stopping at the first newline instead let
 * every multiline named import through unchecked. Requiring the keyword to open
 * a line is still what excludes a dynamic `import()`, which is never written
 * there.
 */
const STATIC_IMPORT_PATTERN = /(?:^|\n)\s*(?:import|export)\b[^;]*?\bfrom\s*['"]([^'"]+)['"]/g;

const RULES = [
  {
    restricted: MOBILE_ONLY,
    side: MOBILE_SIDE,
    exempt: CAPACITOR_EXEMPT,
    problem: 'Shared code must not import mobile-only modules:',
    hint:
      'Reach the mobile stack through src/shells/mobile instead — add a RuntimeShell\n' +
      'member (src/providers/shell.ts) and let the shell register it.'
  },
  {
    restricted: READER_PRIVATE,
    side: READER_SIDE,
    exempt: new Map(),
    problem: 'Only the reader may import reader modules:',
    hint:
      'The reader is a feature, not a library. Move what more than one feature needs\n' +
      'into src/utils or src/components — safe HTML rendering lives in\n' +
      'src/utils/safeHtml.ts and src/components/SafeHtml.vue.'
  }
];

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

/**
 * Returns each rule alongside the imports that broke it, for `base`, the
 * directory holding `src`. Separated from the reporting below so the rules can
 * be exercised against fixtures: a boundary rule that matches nothing looks
 * exactly like a boundary nobody crosses.
 */
export async function findViolations(base = '.') {
  const sources = [];
  for await (const path of walk(join(base, SRC))) {
    sources.push([relative(base, path).replaceAll('\\', '/'), await readFile(path, 'utf8')]);
  }

  return RULES.map((rule) => {
    const violations = [];

    for (const [file, source] of sources) {
      if (rule.side.some((pattern) => pattern.test(file))) {
        continue;
      }

      const exempt = rule.exempt.get(file) ?? new Set();

      for (const match of source.matchAll(STATIC_IMPORT_PATTERN)) {
        const specifier = match[1];
        if (exempt.has(specifier)) {
          continue;
        }
        if (rule.restricted.some((restricted) => restricted.test(specifier))) {
          violations.push(`${file} imports ${specifier}`);
        }
      }
    }

    return { rule, violations };
  });
}

async function main() {
  const results = await findViolations();
  let failed = false;

  for (const { rule, violations } of results) {
    if (violations.length === 0) {
      continue;
    }

    failed = true;
    console.error(`${rule.problem}\n`);
    for (const violation of violations) {
      console.error(`  ${violation}`);
    }
    console.error(`\n${rule.hint}\n`);
  }

  if (failed) {
    process.exit(1);
  }

  console.log(`Module boundaries OK (${RULES.length} rules).`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
