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

function tokens(dark = DARK, systemDark = DARK) {
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

  it('exempts the reading surface', async () => {
    const css = '.reader-text { color: #3f3529; }';
    expect(await findRawColors(await tree({ 'src/features/reader/styles/r.css': css }))).toEqual([]);
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

describe('contrastRatio', () => {
  it('matches the WCAG extremes', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrastRatio('#777', '#777777')).toBeCloseTo(1, 5);
  });
});
