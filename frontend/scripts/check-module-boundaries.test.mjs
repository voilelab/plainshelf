import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import { findViolations } from './check-module-boundaries.mjs';

let base = null;

/** Writes a `src` tree and returns what each rule found in it, rule order kept. */
async function check(files) {
  base = await mkdtemp(join(tmpdir(), 'boundaries-'));
  for (const [path, source] of Object.entries(files)) {
    await mkdir(dirname(join(base, path)), { recursive: true });
    await writeFile(join(base, path), source);
  }
  const results = await findViolations(base);
  return results.map(({ violations }) => violations);
}

afterEach(async () => {
  if (base) await rm(base, { recursive: true, force: true });
  base = null;
});

describe('findViolations', () => {
  it('reports nothing for shared code that keeps to its side', async () => {
    expect(
      await check({
        'src/utils/safeHtml.ts': "import createSanitizer from 'dompurify';\n",
        'src/features/reader/components/ReaderSafeHtml.vue':
          "<script setup lang=\"ts\">\nimport SafeHtml from '@/components/SafeHtml.vue';\n</script>\n"
      })
    ).toEqual([[], []]);
  });

  // The repository writes named imports across several lines as often as one.
  // A pattern that stopped at the first newline reported every file below clean.
  it('catches a multiline import on either side of a boundary', async () => {
    expect(
      await check({
        'src/utils/probe.ts': [
          'import {',
          '  renderMarkdownBlocks',
          "} from '@/features/reader/utils/renderMarkdownBlocks';",
          'import {',
          '  Preferences',
          "} from '@capacitor/preferences';",
          ''
        ].join('\n')
      })
    ).toEqual([
      ['src/utils/probe.ts imports @capacitor/preferences'],
      ['src/utils/probe.ts imports @/features/reader/utils/renderMarkdownBlocks']
    ]);
  });

  it('catches a multiline re-export and does not let a side-effect import hide one', async () => {
    expect(
      await check({
        'src/utils/reexport.ts': [
          'export type Marker = string;',
          'export {',
          '  renderMarkdownBlocks',
          "} from '@/features/reader/utils/renderMarkdownBlocks';",
          ''
        ].join('\n'),
        'src/utils/after.ts': [
          "import '@/styles.css';",
          'import {',
          '  Preferences',
          "} from '@capacitor/preferences';",
          ''
        ].join('\n')
      })
    ).toEqual([
      ['src/utils/after.ts imports @capacitor/preferences'],
      ['src/utils/reexport.ts imports @/features/reader/utils/renderMarkdownBlocks']
    ]);
  });

  it('leaves dynamic imports alone: they are how the router reaches both', async () => {
    expect(
      await check({
        'src/router.ts': [
          "import { createRouter } from 'vue-router';",
          "const ReaderPage = () => import('@/features/reader/pages/ReaderView.vue');",
          "const MobilePage = () => import('@/features/mobile/MobileHome.vue');",
          'export const routes = [ReaderPage, MobilePage, createRouter];',
          ''
        ].join('\n')
      })
    ).toEqual([[], []]);
  });

  it('keeps the reader and the mobile shell free inside their own side', async () => {
    expect(
      await check({
        'src/features/reader/composables/useReader.ts':
          "import { renderMarkdownBlocks } from '@/features/reader/utils/renderMarkdownBlocks';\n",
        'src/shells/mobile/index.ts': "import { Preferences } from '@capacitor/preferences';\n"
      })
    ).toEqual([[], []]);
  });

  it('honors the runtime.ts exemption without widening it', async () => {
    expect(
      await check({
        'src/providers/runtime.ts': [
          "import { Capacitor } from '@capacitor/core';",
          "import { Preferences } from '@capacitor/preferences';",
          'export const probe = [Capacitor, Preferences];',
          ''
        ].join('\n')
      })
    ).toEqual([['src/providers/runtime.ts imports @capacitor/preferences'], []]);
  });

  it('skips unit tests, which may import anything they exercise', async () => {
    expect(
      await check({
        'src/utils/probe.test.ts':
          "import { renderMarkdownBlocks } from '@/features/reader/utils/renderMarkdownBlocks';\n"
      })
    ).toEqual([[], []]);
  });
});
