/**
 * Fails when a module exports a name no other module uses.
 *
 * PSW-60 cut the count of such exports from 187 to 57 by hand, and left nothing
 * to stop it growing back. This is that ratchet: a module's exports are its
 * promise to the rest of the app, and an export only its own file (or only its
 * own test) reads is not a promise, it is a leaked internal that the next reader
 * has to treat as load-bearing.
 *
 * Detection is deliberately string-based, not AST-based: a name counts as used
 * when it appears as a word anywhere in another file, including in a comment or
 * as an unrelated identifier that happens to share the name. That direction is
 * the point — a false positive blocks someone else's PR, a false negative only
 * leaves work for the next pass.
 *
 * Two consequences of the same choice, both intended:
 *   - `const { X } = await import('./m')` and `type X = import('./m').Y` count as
 *     uses without any special case. `main.ts` loads `installMobileShell` that way.
 *   - only declaration forms are scanned (`export const|function|class|type|…`).
 *     `export { X }` and `export * from` are re-exports whose name also appears
 *     at its declaration, so scanning them would report the barrel, not the
 *     module that owns the name.
 *
 * Exemptions are a flat allowlist below rather than an inline pragma, so the
 * cost of keeping one is visible in a single place and its size is reviewable.
 * A stale entry is an error too: the list can shrink on its own, never rot.
 */
import { readdir, readFile } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { pathToFileURL } from 'node:url';

const SRC = 'src';

/** `export [declare] [async] <kind> NAME`, the forms PSW-60 counted. */
const EXPORTED_DECLARATION =
  /^export\s+(?:declare\s+)?(?:async\s+)?(?:function|const|let|var|class|interface|type|enum)\s+([A-Za-z_$][\w$]*)/gm;

/**
 * Exports kept although nothing outside their own file reads them, as
 * `path#Name`. Almost all are internals a unit test reaches directly; each one
 * is a deliberate testing seam, not a module boundary, and the honest way to
 * retire it is to test the module through the surface its real callers use.
 *
 * Adding an entry is allowed. Adding one without reading the two lines above is
 * how the count gets back to 187.
 */
const ALLOWED = new Set([
  // A contract other callers are written against, stated in its own file even
  // though only this file's own `extends` names it.
  'src/features/sources/types/editorAdapter.ts#SourceEditorAdapter',

  // Read from outside the frontend: e2e/tests/support/locale.ts imports the
  // pattern by relative path so the crawl and its unit test share one
  // definition, and this script only walks src/.
  'src/i18n/missingKeyPattern.ts#MISSING_KEY_PATTERN',

  // The shelf's adult-content rules, which this reader computes so that the
  // cross-language conformance dataset can pin the answer (PSW-116). Nothing on
  // the pCloud side applies the mark yet; the client that does is what will make
  // these ordinary exports.
  'src/api/pcloud/bookpkg.ts#createNSFWRules',
  'src/api/pcloud/bookpkg.ts#isBookNSFW',

  // Internals reached directly by their own unit tests.
  'src/api/books.ts#SimilarRelation',
  'src/api/pcloud/auth.ts#hostForLocationId',
  'src/api/pcloud/bookCacheFile.ts#BOOK_CACHE_SCHEMA_VERSION',
  'src/api/pcloud/bookpkg.ts#BOOK_META_SCHEMA_VERSION',
  'src/api/pcloud/bookpkg.ts#DEFAULT_IGNORE_RULES',
  'src/api/pcloud/bookpkg.ts#isSchemaNewerThanSupported',
  'src/composables/shelfInitRetry.ts#SHELF_INIT_MAX_AUTO_RETRIES',
  'src/composables/shelfInitRetry.ts#SHELF_INIT_RETRY_DELAY_MS',
  'src/composables/useFolderManagement.ts#createdFolderDestination',
  'src/composables/useFolderManagement.ts#movedFolderDestination',
  'src/composables/useFolderManagement.ts#renamedFolderDestination',
  'src/composables/useReaderLaunchPreference.ts#DEFAULT_READER_LAUNCH_MODE',
  'src/composables/useReaderLaunchPreference.ts#parseReaderLaunchMode',
  'src/composables/useSafeBackNavigation.ts#isSafePlainShelfBackTarget',
  'src/composables/useSafeBackNavigation.ts#navigateBackSafely',
  'src/composables/useUnsavedChangesGuard.ts#historyTraversalDirection',
  'src/features/library/composables/useImportBook.ts#ImportUnit',
  'src/features/library/utils/bookDetail.ts#normalizeReadingPercent',
  'src/features/library/utils/mobileBack.ts#MobileBackActions',
  'src/features/mobile/composables/usePCloudFolderBrowser.ts#PCloudFolderClient',
  'src/features/mobile/composables/usePCloudFolderBrowser.ts#PCloudFolderRef',
  'src/features/reader/composables/useReaderPresentation.ts#shouldUseMobileReader',
  'src/features/reader/composables/useReaderSettings.ts#DEFAULT_READER_FONT',
  'src/features/reader/composables/useReaderSettings.ts#parseReaderFont',
  'src/features/reader/composables/useReadingProgressAutosave.ts#READING_PROGRESS_AUTOSAVE_INTERVAL_MS',
  'src/features/settings/utils/scanInterval.ts#parseGoDuration',
  'src/features/sources/utils/textEditing.ts#clampTextOffset',
  'src/providers/index.ts#createBookshelfProvider',
  'src/providers/mobileBookshelfProvider.ts#DOWNLOAD_SHELF_CHANGED_ERROR',
  'src/providers/mobileCacheFs.ts#UNSCOPED_DIR_NAME',
  'src/providers/mobileConfig.ts#ServerShelfEntry',
  'src/providers/pcloudBookshelfProvider.ts#pcloudCoverUrl',
  'src/providers/shelfSnapshotStore.ts#parseShelfSnapshot',
  'src/shells/reader/routerGuard.ts#READER_EMPTY_ROUTE',
  'src/storage/readHistory/index.ts#ReadHistoryStore',
  'src/storage/readHistory/index.ts#buildReadHistoryKey',
  'src/storage/readHistory/storage.ts#READ_HISTORY_STORAGE_KEY',
  'src/storage/readingProgress/document.ts#READING_PROGRESS_DOCUMENT_VERSION',
  'src/storage/readingProgress/index.ts#ReadingProgressStore',
  'src/storage/readingProgress/index.ts#buildReadingProgressKey',
  'src/storage/readingProgress/storage.ts#READING_PROGRESS_STORAGE_KEY',
  'src/storage/readingStats/document.ts#MAX_SECONDS_PER_CALL',
  'src/storage/readingStats/document.ts#MAX_SECONDS_PER_DAY',
  'src/storage/readingStats/document.ts#READING_STATS_DOCUMENT_VERSION',
  'src/storage/readingStats/index.ts#ReadingStatsStore',
  'src/utils/bookFilters/registry.ts#authorFilter',
  'src/utils/bookFilters/registry.ts#coverFilter',
  'src/utils/bookFilters/registry.ts#languageFilter',
  'src/utils/bookFilters/registry.ts#tagsFilter',
  'src/utils/charCountFilter.ts#parseCharCountBound',
  'src/utils/folders.ts#ROOT_FOLDER_FILTER',
  'src/utils/folders.ts#flattenFolderTreePaths',
  'src/utils/safeHtml.ts#sanitizeColorStyle',
  'src/utils/sidebarMode.ts#SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY',
  'src/utils/sidebarMode.ts#SIDEBAR_MODE_STORAGE_KEY',
  'src/utils/sidebarMode.ts#isSidebarMode',
  'src/utils/similarity.ts#SIMILARITY_SHINGLE_K',
  'src/utils/similarity.ts#estimatedDiffRate'
]);

const isTest = (file) => /\.test\.ts$/.test(file);

/**
 * Word-like runs, matching what `\bNAME\b` would find. `$` is a non-word
 * character to `\b`, so it splits tokens here too; an export whose name contains
 * one is therefore skipped rather than mis-scanned (see `declarationsIn`).
 */
function identifiersIn(source) {
  return new Set(source.match(/\w+/g) ?? []);
}

/** Exported declarations in `source`, with the 1-based line each opens on. */
function declarationsIn(source) {
  const declarations = [];
  for (const match of source.matchAll(EXPORTED_DECLARATION)) {
    const name = match[1];
    if (name.includes('$')) {
      continue;
    }
    const line = source.slice(0, match.index).split('\n').length;
    declarations.push({ name, line });
  }
  return declarations;
}

async function* walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      yield* walk(path);
    } else if (/\.(ts|vue)$/.test(entry.name)) {
      yield path;
    }
  }
}

/**
 * Exports under `base` that no other non-test file names, plus the allowlist
 * entries that no longer match one. `allowed` is a parameter so the rule can be
 * exercised against fixtures without the repository's own list.
 */
export async function findUnusedExports(base = '.', allowed = ALLOWED) {
  const files = [];
  for await (const path of walk(join(base, SRC))) {
    const file = relative(base, path).replaceAll('\\', '/');
    files.push({ file, source: await readFile(path, 'utf8') });
  }
  for (const entry of files) {
    entry.identifiers = identifiersIn(entry.source);
  }

  const unused = [];
  const matched = new Set();

  for (const { file, source } of files) {
    if (isTest(file)) {
      continue;
    }
    for (const { name, line } of declarationsIn(source)) {
      const others = files.filter((other) => other.file !== file && other.identifiers.has(name));
      if (others.some((other) => !isTest(other.file))) {
        continue;
      }

      const key = `${file}#${name}`;
      if (allowed.has(key)) {
        matched.add(key);
        continue;
      }
      unused.push({ file, name, line, tests: others.length });
    }
  }

  unused.sort((a, b) => a.file.localeCompare(b.file) || a.line - b.line);
  return { unused, stale: [...allowed].filter((key) => !matched.has(key)).sort() };
}

const HINT =
  'Drop the `export` and keep the name module-private. If a unit test is the only\n' +
  'reader, prefer testing the module through the surface its real callers use; when\n' +
  'the seam is genuinely worth keeping, add the entry to ALLOWED in this script.';

async function main() {
  const { unused, stale } = await findUnusedExports();

  if (unused.length > 0) {
    console.error('Exports no other module uses:\n');
    for (const { file, name, line, tests } of unused) {
      const by = tests === 0 ? 'nothing references it' : `only ${tests} test file(s) reference it`;
      console.error(`  ${file}:${line} exports ${name} — ${by}`);
    }
    console.error(`\n${HINT}\n`);
  }

  if (stale.length > 0) {
    console.error('ALLOWED entries that no longer match an unused export:\n');
    for (const key of stale) {
      console.error(`  ${key}`);
    }
    console.error('\nThe export gained a real caller or is gone. Delete the entry.\n');
  }

  if (unused.length > 0 || stale.length > 0) {
    process.exit(1);
  }

  console.log(`Exports OK (${ALLOWED.size} allowlisted).`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
