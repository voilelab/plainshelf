import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import { contrastRatio, findRawColors, findThemeViolations } from './theme-colors.mjs';

const DARK = `
  --bg: #11161d;
  --surface: #1a212b;
  --text: #e4e9f0;
  --text-secondary: #bac4d1;
  --muted: #95a1b2;
  --accent: #2f72db;
  --on-accent: #ffffff;
  --danger: #c93c37;
  --danger-text: #ffa198;
  --danger-soft: #3a1e20;
`;

const READER_DARK = `
  --reader-paper: #1e2024;
  --reader-ink: #dad5c8;
  --reader-heading: #ece6d8;
  --reader-ink-soft: #c4beb0;
  --reader-muted: #a7a194;
`;

function readerTokens(readerDark = READER_DARK) {
  return `
.reading-surface {
  --reader-paper: #fffdf7;
  --reader-ink: #2f2a22;
  --reader-heading: #3f3529;
  --reader-ink-soft: #4f4639;
  --reader-muted: #6b5f4a;
  --comic-bg: #181818;
}

.reading-surface[data-reader-theme='sepia'] {
  --reader-paper: #f6ecd5;
  --reader-ink: #3b2e1e;
  --reader-heading: #47371f;
  --reader-ink-soft: #57442c;
  --reader-muted: #6b5638;
}

.reading-surface[data-reader-theme='dark'] {${readerDark}}
`;
}

function tokens(dark = DARK, systemDark = DARK, readerDark = READER_DARK) {
  return `:root {
  --bg: #f4f6f8;
  --surface: #ffffff;
  --text: #142033;
  --text-secondary: #475569;
  --muted: #66758a;
  --accent: #1f6feb;
  --on-accent: #ffffff;
  --danger: #cf3d3d;
  --danger-text: #991b1b;
  --danger-soft: #fef2f2;
  --z-modal: 80;
}

:root[data-theme='dark'] {${dark}}

@media (prefers-color-scheme: dark) {
  :root:not([data-theme]) {${systemDark}}
}

${readerTokens(readerDark)}
body { background: var(--bg); }
`;
}

let base = null;

async function tree(files) {
  base = await mkdtemp(join(tmpdir(), 'theme-colors-'));
  for (const [path, source] of Object.entries({ 'src/styles.css': tokens(), ...files })) {
    await mkdir(dirname(join(base, path)), { recursive: true });
    await writeFile(join(base, path), source);
  }
  return base;
}

afterEach(async () => {
  if (base) await rm(base, { recursive: true, force: true });
  base = null;
});

describe('findRawColors', () => {
  it('accepts colors that only come from tokens', async () => {
    expect(await findRawColors(await tree({ 'src/a.css': '.a { color: var(--text); }' }))).toEqual([]);
  });

  it('reports a hex, an rgba and a named color in a component style', async () => {
    const found = await findRawColors(
      await tree({
        'src/components/A.vue': `<template><p class="a" /></template>
<style scoped>
.a { color: #fff; box-shadow: 0 1px 2px rgba(15, 23, 42, 0.2); }
.b { background: color-mix(in srgb, var(--accent) 8%, white); white-space: nowrap; }
</style>`
      })
    );
    expect(found).toEqual([
      'src/components/A.vue: .a { color: #fff }',
      'src/components/A.vue: .a { box-shadow: 0 1px 2px rgba(15, 23, 42, 0.2) }',
      'src/components/A.vue: .b { background: color-mix(in srgb, var(--accent) 8%, white) }'
    ]);
  });

  it('allows channel tokens inside rgb()', async () => {
    const css = '.a { box-shadow: 0 1px 2px rgb(var(--shadow-rgb) / 0.2); }';
    expect(await findRawColors(await tree({ 'src/a.css': css }))).toEqual([]);
  });

  it('ignores colors in a component script and in comments', async () => {
    const vue = `<script setup>const ink = '#1a1a2e';</script>
<style>/* was #fff */ .a { color: var(--text); }</style>`;
    expect(await findRawColors(await tree({ 'src/A.vue': vue }))).toEqual([]);
  });

  it('reports a literal color on the reading surface', async () => {
    const css = '.reader-text { color: #3f3529; }';
    expect(await findRawColors(await tree({ 'src/features/reader/styles/r.css': css }))).toEqual([
      'src/features/reader/styles/r.css: .reader-text { color: #3f3529 }'
    ]);
  });

  it('allows reader token definitions on .reading-surface in the token file', async () => {
    expect(await findRawColors(await tree({}))).toEqual([]);
  });

  it('allows a literal only as a token definition in the token file', async () => {
    const found = await findRawColors(
      await tree({ 'src/other.css': ':root { --bg: #000000; }', 'src/styles.css': tokens() + '.x { --bg: #000; }' })
    );
    expect(found).toEqual(['src/other.css: :root { --bg: #000000 }', 'src/styles.css: .x { --bg: #000 }']);
  });
});

describe('findThemeViolations', () => {
  it('accepts complete, readable light and dark themes', async () => {
    expect(await findThemeViolations(await tree({}))).toEqual([]);
  });

  it('reports the system dark block drifting from the explicit one', async () => {
    const found = await findThemeViolations(
      await tree({ 'src/styles.css': tokens(DARK, DARK.replace('#e4e9f0', '#ffffff')) })
    );
    expect(found).toEqual([`:root:not([data-theme]) --text is "#ffffff", :root[data-theme='dark'] has "#e4e9f0"`]);
  });

  it('reports a light token with no dark value', async () => {
    const found = await findThemeViolations(await tree({ 'src/styles.css': tokens(DARK.replace(/ {2}--muted.*\n/, '')) }));
    expect(found).toContain(':root defines --muted with no dark value');
  });

  it('reports dark body text below WCAG AA', async () => {
    const dim = DARK.replace('--text: #e4e9f0', '--text: #4a5361');
    const found = await findThemeViolations(await tree({ 'src/styles.css': tokens(dim, dim) }));
    expect(found.some((line) => line.startsWith('dark: --text #4a5361 on --bg'))).toBe(true);
  });

  it('reports an accent that disappears into the dark background', async () => {
    const dim = DARK.replace('--accent: #2f72db', '--accent: #1c2a40');
    const found = await findThemeViolations(await tree({ 'src/styles.css': tokens(dim, dim) }));
    expect(found.some((line) => line.startsWith('dark: --accent #1c2a40 on --bg'))).toBe(true);
  });
});

describe('reader themes', () => {
  it('reports a reader theme missing a reader token', async () => {
    const found = await findThemeViolations(
      await tree({ 'src/styles.css': tokens(DARK, DARK, READER_DARK.replace(/ {2}--reader-muted.*\n/, '')) })
    );
    expect(found).toContain(".reading-surface[data-reader-theme='dark'] has no --reader-muted");
  });

  it('reports reader muted text below WCAG AA on its paper', async () => {
    const dim = READER_DARK.replace('--reader-muted: #a7a194', '--reader-muted: #4a4740');
    const found = await findThemeViolations(await tree({ 'src/styles.css': tokens(DARK, DARK, dim) }));
    expect(found.some((line) => line.startsWith('reader dark: --reader-muted #4a4740 on --reader-paper'))).toBe(
      true
    );
  });

  it('reports a missing reader theme block', async () => {
    const css = tokens().replace(/\.reading-surface\[data-reader-theme='sepia'\] \{[^}]*\}/, '');
    const found = await findThemeViolations(await tree({ 'src/styles.css': css }));
    expect(found).toEqual([
      "src/styles.css needs reader token blocks: .reading-surface[data-reader-theme='sepia']"
    ]);
  });
});

describe('contrastRatio', () => {
  it('matches the WCAG extremes', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrastRatio('#777', '#777777')).toBeCloseTo(1, 5);
  });
});
