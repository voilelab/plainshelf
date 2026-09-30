import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import { findUnusedExports } from './check-exports.mjs';

let base = null;

/** Writes a `src` tree and runs the rule over it with `allowed` as the allowlist. */
async function check(files, allowed = []) {
  base = await mkdtemp(join(tmpdir(), 'exports-'));
  for (const [path, source] of Object.entries(files)) {
    await mkdir(dirname(join(base, path)), { recursive: true });
    await writeFile(join(base, path), source);
  }
  return findUnusedExports(base, new Set(allowed));
}

afterEach(async () => {
  if (base) await rm(base, { recursive: true, force: true });
  base = null;
});

describe('findUnusedExports', () => {
  it('reports nothing when another module imports the export', async () => {
    const { unused, stale } = await check({
      'src/utils/folders.ts': 'export function flattenFolders() {}\n',
      'src/features/library/LibraryPage.vue':
        "<script setup lang=\"ts\">\nimport { flattenFolders } from '@/utils/folders';\nflattenFolders();\n</script>\n"
    });
    expect(unused).toEqual([]);
    expect(stale).toEqual([]);
  });

  it('flags an export only its own file uses, with its line', async () => {
    const { unused } = await check({
      'src/utils/folders.ts': [
        'export type FolderKind = string;',
        'export function label(kind: FolderKind) {',
        '  return kind;',
        '}',
        ''
      ].join('\n'),
      'src/pages/Home.vue': "<script setup lang=\"ts\">\nimport { label } from '@/utils/folders';\nlabel('a');\n</script>\n"
    });
    expect(unused).toEqual([{ file: 'src/utils/folders.ts', name: 'FolderKind', line: 1, tests: 0 }]);
  });

  it('flags an export only a test imports, and counts the tests', async () => {
    const { unused } = await check({
      'src/utils/similarity.ts': 'export const SHINGLE_K = 5;\nexport function score() {}\n',
      'src/utils/similarity.test.ts': "import { SHINGLE_K, score } from './similarity';\nvoid [SHINGLE_K, score];\n",
      'src/pages/Home.vue': "<script setup lang=\"ts\">\nimport { score } from '@/utils/similarity';\nscore();\n</script>\n"
    });
    expect(unused).toEqual([{ file: 'src/utils/similarity.ts', name: 'SHINGLE_K', line: 1, tests: 1 }]);
  });

  it('honors the allowlist and reports nothing for the entry it covers', async () => {
    const { unused, stale } = await check(
      { 'src/utils/similarity.ts': 'export const SHINGLE_K = 5;\n' },
      ['src/utils/similarity.ts#SHINGLE_K']
    );
    expect(unused).toEqual([]);
    expect(stale).toEqual([]);
  });

  // The allowlist can shrink on its own: an entry whose export gained a real
  // caller, or was deleted, fails rather than sitting there forever.
  it('reports an allowlist entry that no longer matches an unused export', async () => {
    const { unused, stale } = await check(
      {
        'src/utils/similarity.ts': 'export const SHINGLE_K = 5;\n',
        'src/pages/Home.vue': "<script setup lang=\"ts\">\nimport { SHINGLE_K } from '@/utils/similarity';\nvoid SHINGLE_K;\n</script>\n"
      },
      ['src/utils/similarity.ts#SHINGLE_K', 'src/utils/gone.ts#Removed']
    );
    expect(unused).toEqual([]);
    expect(stale).toEqual(['src/utils/gone.ts#Removed', 'src/utils/similarity.ts#SHINGLE_K']);
  });

  // main.ts loads installMobileShell this way, and the sources page names its
  // adapter through an import type. Both must read as uses.
  it('counts a dynamic import and an import type as uses', async () => {
    const { unused } = await check({
      'src/shells/mobile/index.ts': 'export function installMobileShell() {}\n',
      'src/features/sources/types/editorAdapter.ts': 'export interface SourceEditorAdapter {\n  jump(): void;\n}\n',
      'src/main.ts': [
        "const { installMobileShell } = await import('./shells/mobile');",
        "type Adapter = import('./features/sources/types/editorAdapter').SourceEditorAdapter;",
        'export const boot = [installMobileShell, null as unknown as Adapter];',
        ''
      ].join('\n'),
      'src/boot.ts': "import { boot } from './main';\nvoid boot;\n"
    });
    expect(unused).toEqual([]);
  });

  // Word boundaries, not substrings: createShelfInitRetry must not mark the
  // ShelfInitRetry type as used, or the check silently passes everything.
  it('does not count a longer identifier that contains the name', async () => {
    const { unused } = await check({
      'src/composables/shelfInitRetry.ts': [
        'export interface ShelfInitRetry {',
        '  attempts: number;',
        '}',
        'export function createShelfInitRetry(): ShelfInitRetry {',
        '  return { attempts: 0 };',
        '}',
        ''
      ].join('\n'),
      'src/composables/useBookStore.ts':
        "import { createShelfInitRetry } from './shelfInitRetry';\ncreateShelfInitRetry();\n"
    });
    expect(unused).toEqual([
      { file: 'src/composables/shelfInitRetry.ts', name: 'ShelfInitRetry', line: 1, tests: 0 }
    ]);
  });

  // Only declaration forms are scanned: a barrel's `export { X } from` would
  // otherwise be reported instead of the module that owns the name.
  it('ignores re-export forms and scans no declarations inside a test file', async () => {
    const { unused } = await check({
      'src/storage/index.ts': "export { readHistoryStore } from './readHistory';\nexport * from './readingStats';\n",
      'src/storage/readHistory.ts': 'export const readHistoryStore = {};\n',
      'src/storage/readingStats.test.ts': 'export const helperOnlyATestDeclares = 1;\n'
    });
    expect(unused).toEqual([]);
  });
});
