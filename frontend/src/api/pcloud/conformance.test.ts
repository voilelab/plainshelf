// The dataset is a directory tree outside this project, so this file reads it
// with Node's filesystem API. The reference pulls in the types for that: the
// app's tsconfig declares only browser-side type packages, which is what keeps
// `node:` imports out of shipped code, and this test is not shipped.
/// <reference types="node" />
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

import { bookPackagePath, findBookCacheFiles, parseBookCacheFile } from './bookCacheFile';
import type { BookJson, PCloudFileRef, ShelfConfig } from './bookpkg';
import {
  collectBookPackages,
  collectFolders,
  createIgnoreRules,
  createNSFWRules,
  DEFAULT_IGNORED_DIRS,
  findBooksFolder,
  findCoverFile,
  findCurrentSource,
  findShelfConfigFile,
  isBookNSFW,
  isSchemaNewerThanSupported,
  parseBookJson,
  parseShelfConfig,
  toSourceMeta
} from './bookpkg';
import type { PCloudItem } from './types';

/**
 * The pCloud half of the cross-language conformance suite. Its Go twin is
 * `shelf/conformance_test.go`, and both read the same dataset from
 * `shelf/testdata/conformance` — see the README there for the contract.
 *
 * This client re-implements the Go shelf's walk against pCloud's listing API
 * and cannot import any of it, so nothing but this dataset notices when the two
 * drift apart. The unit tests beside it cover this module's own edge cases; what
 * is checked here is only what both implementations must agree on.
 */

/** The expected.json shape this harness understands (schema_version in manifest.json). */
const DATASET_VERSION = 3;

const DATASET_ROOT = fileURLToPath(new URL('../../../../shelf/testdata/conformance/', import.meta.url));

interface CaseManifestEntry {
  name: string;
  covers: string;
}

interface Manifest {
  schema_version: number;
  cases: CaseManifestEntry[];
}

interface ExpectedSource {
  id: string;
  schema_version: number;
  created_at: string;
  comment: string;
  format: string;
  md5_hash: string;
  has_content: boolean;
  line_count: number;
  char_count: number;
  assets: string[];
}

interface ExpectedBook {
  path: string;
  folders: string[];
  id: string;
  title: string;
  format: string;
  authors: string[];
  tags: string[];
  identifiers: Record<string, string>;
  language: string;
  comments: string;
  star: number;
  created_at: string;
  updated_at: string;
  published_at: string;
  cover: string;
  cover_present: boolean;
  schema_version_on_disk: number;
  read_only: boolean;
  nsfw: boolean;
  current_source_field: string;
  current_source: string | null;
  sources: ExpectedSource[];
}

interface ExpectedBookCache {
  name: string;
  usable: boolean;
  book_ids: string[];
}

interface ExpectedReading {
  folders: string[];
  books: ExpectedBook[];
  book_caches: ExpectedBookCache[];
}

function readJsonFile<T>(path: string): T {
  return JSON.parse(readFileSync(path, 'utf8')) as T;
}

/**
 * Turns a case's shelf directory into the recursive listing pCloud would
 * return, plus the file contents that listing only points at.
 *
 * Real ids are irrelevant to what is compared, so they are handed out in walk
 * order; the map back to a real path is what lets the harness "download" a file
 * the way the client does.
 */
function listShelf(shelfDir: string): { root: PCloudItem; contents: Map<number, string> } {
  const contents = new Map<number, string>();
  let nextID = 1;

  const walk = (dir: string, name: string): PCloudItem => {
    const item: PCloudItem = { name, isfolder: true, folderid: nextID++, contents: [] };

    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const entryPath = join(dir, entry.name);
      if (entry.isDirectory()) {
        item.contents?.push(walk(entryPath, entry.name));
        continue;
      }
      const stat = statSync(entryPath);
      const fileid = nextID++;
      contents.set(fileid, entryPath);
      item.contents?.push({
        name: entry.name,
        isfolder: false,
        fileid,
        size: stat.size,
        modified: stat.mtime.toUTCString()
      });
    }

    return item;
  };

  return { root: walk(shelfDir, 'shelf'), contents };
}

function readListedJson<T>(contents: Map<number, string>, ref: PCloudFileRef): T {
  const path = contents.get(ref.fileid);
  if (!path) {
    throw new Error(`listing has no content for file ${ref.name}`);
  }
  return readJsonFile<T>(path);
}

function readCase(shelfDir: string): ExpectedReading {
  const { root, contents } = listShelf(shelfDir);

  const booksFolder = findBooksFolder(root);
  if (!booksFolder) {
    throw new Error(`${shelfDir} has no books folder`);
  }

  // The shelf's own settings decide which directories are skipped, so they are
  // read before the walk, exactly as the provider does.
  const configRef = findShelfConfigFile(root);
  const config: ShelfConfig = configRef ? parseShelfConfig(readListedJson(contents, configRef)) : {};
  const ignore = createIgnoreRules(config.ignoredDirs ?? DEFAULT_IGNORED_DIRS);
  const isNSFWFolder = createNSFWRules(config.nsfwFolders ?? []);

  const books: ExpectedBook[] = [];
  for (const pkg of collectBookPackages(booksFolder, ignore)) {
    if (!pkg.meta) {
      // A package with no book.json is skipped by the provider that consumes
      // this walk, and the dataset deliberately holds none — see the README.
      continue;
    }
    const meta: BookJson = parseBookJson(readListedJson(contents, pkg.meta));

    books.push({
      path: bookPackagePath(pkg),
      folders: pkg.folders,
      id: meta.id,
      title: meta.title,
      format: meta.format ?? '',
      authors: meta.authors ?? [],
      tags: meta.tags ?? [],
      identifiers: meta.identifiers ?? {},
      language: meta.language ?? '',
      comments: meta.comments ?? '',
      star: meta.star ?? 0,
      created_at: meta.created_at ?? '',
      updated_at: meta.updated_at ?? '',
      published_at: meta.published_at ?? '',
      cover: meta.cover ?? '',
      cover_present: findCoverFile(pkg, meta) !== undefined,
      schema_version_on_disk: meta.schema_version ?? 0,
      read_only: isSchemaNewerThanSupported(meta),
      nsfw: isBookNSFW(isNSFWFolder, pkg.folders, meta),
      current_source_field: meta.current_source ?? '',
      current_source: findCurrentSource(pkg, meta)?.id ?? null,
      sources: pkg.sources.map((source) => {
        const sourceMeta = source.meta
          ? toSourceMeta(readListedJson(contents, source.meta), source.id)
          : undefined;
        return {
          id: source.id,
          schema_version: sourceMeta?.schema_version ?? 0,
          created_at: sourceMeta?.created_at ?? '',
          comment: sourceMeta?.comment ?? '',
          format: sourceMeta?.format ?? '',
          md5_hash: sourceMeta?.md5_hash ?? '',
          has_content: source.content !== undefined,
          line_count: sourceMeta?.line_count ?? 0,
          char_count: sourceMeta?.char_count ?? 0,
          assets: Object.keys(source.assets).sort()
        };
      })
    });
  }
  books.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));

  const bookCaches: ExpectedBookCache[] = findBookCacheFiles(root).map((file) => {
    const cache = parseBookCacheFile(readListedJson(contents, file));
    return {
      name: file.name,
      usable: cache !== null,
      book_ids: cache ? Object.keys(cache.books).sort() : []
    };
  });
  bookCaches.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));

  return { folders: collectFolders(booksFolder, ignore), books, book_caches: bookCaches };
}

const manifest = readJsonFile<Manifest>(join(DATASET_ROOT, 'manifest.json'));

describe('bookpkg conformance dataset', () => {
  it('is the version this harness reads', () => {
    expect(manifest.schema_version).toBe(DATASET_VERSION);
    expect(manifest.cases.length).toBeGreaterThan(0);
  });

  // Without this a case could be added and never run here, which is the one
  // failure mode a conformance suite cannot afford: it would report agreement
  // it never checked.
  it('lists every case directory and nothing else', () => {
    const listed = manifest.cases.map((entry) => entry.name).sort();
    const present = readdirSync(join(DATASET_ROOT, 'cases'), { withFileTypes: true })
      .map((entry) => {
        expect(entry.isDirectory()).toBe(true);
        return entry.name;
      })
      .sort();

    expect(listed).toEqual(present);
    for (const entry of manifest.cases) {
      expect(entry.covers, `case ${entry.name} has no "covers" line`).toBeTruthy();
    }
  });

  it('gives every case a shelf and an expected reading', () => {
    for (const entry of manifest.cases) {
      const children = readdirSync(join(DATASET_ROOT, 'cases', entry.name)).sort();
      expect(children, `case ${entry.name}`).toEqual(['expected.json', 'shelf']);
    }
  });
});

describe.each(manifest.cases)('bookpkg conformance: $name', ({ name }) => {
  it('reads the shelf as the dataset expects', () => {
    const caseDir = join(DATASET_ROOT, 'cases', name);
    const expected = readJsonFile<ExpectedReading>(join(caseDir, 'expected.json'));

    expect(readCase(join(caseDir, 'shelf'))).toEqual(expected);
  });
});
