import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';

import { rules, styleSource } from './css-rules.mjs';

export const TOKEN_FILE = 'src/styles.css';

export const DARK_SELECTOR = ":root[data-theme='dark']";
export const SYSTEM_DARK_SELECTOR = ':root:not([data-theme])';

export const READER_THEME_SELECTORS = {
  light: '.reading-surface',
  sepia: ".reading-surface[data-reader-theme='sepia']",
  dark: ".reading-surface[data-reader-theme='dark']"
};

const AA_TEXT = 4.5;
const DISTINGUISHABLE = 3;

export const CONTRAST_PAIRS = [
  { fg: '--text', bg: '--bg', min: AA_TEXT },
  { fg: '--text', bg: '--surface', min: AA_TEXT },
  { fg: '--text-secondary', bg: '--surface', min: AA_TEXT },
  { fg: '--muted', bg: '--surface', min: AA_TEXT },
  { fg: '--on-accent', bg: '--accent', min: AA_TEXT },
  { fg: '--on-accent', bg: '--danger', min: AA_TEXT },
  { fg: '--danger-text', bg: '--danger-soft', min: AA_TEXT },
  { fg: '--accent', bg: '--bg', min: DISTINGUISHABLE },
  { fg: '--accent', bg: '--surface', min: DISTINGUISHABLE },
  { fg: '--danger', bg: '--bg', min: DISTINGUISHABLE },
  { fg: '--danger', bg: '--surface', min: DISTINGUISHABLE }
];

const READER_CONTRAST_PAIRS = ['--reader-ink', '--reader-heading', '--reader-ink-soft', '--reader-muted'].map(
  (fg) => ({ fg, bg: '--reader-paper', min: AA_TEXT })
);

const LITERAL_COLOR =
  /#[0-9a-fA-F]{3,8}\b|\b(?:rgb|hsl)a?\(\s*[\d.]|\b(?:white|black|red|green|blue|gray|grey|silver|orange|yellow|purple|pink|brown)\b/;

async function styleFiles(base, dir = 'src') {
  const found = [];
  for (const entry of await readdir(join(base, dir), { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      found.push(...(await styleFiles(base, path)));
    } else if (/\.(css|vue)$/.test(entry.name)) {
      found.push(path);
    }
  }
  return found;
}

function isTokenDefinition(file, selector, property) {
  return (
    file === TOKEN_FILE &&
    property.startsWith('--') &&
    (selector.includes(':root') || selector.trim().startsWith('.reading-surface'))
  );
}

export async function findRawColors(base = '.') {
  const violations = [];
  for (const file of await styleFiles(base)) {
    const css = styleSource(await readFile(join(base, file), 'utf8'), file);
    for (const rule of rules(css)) {
      for (const [, property, value] of rule.body.matchAll(/([\w-]+)\s*:\s*([^;]+)/g)) {
        if (LITERAL_COLOR.test(value) && !isTokenDefinition(file, rule.selector, property)) {
          violations.push(`${file}: ${rule.selector} { ${property}: ${value.trim()} }`);
        }
      }
    }
  }
  return violations;
}

function tokenBlock(css, selector, { whole = false } = {}) {
  const rule = rules(css).find((candidate) =>
    whole
      ? candidate.selector.trim() === selector
      : candidate.selector.split(',').some((part) => part.trim() === selector)
  );
  if (!rule) return null;
  return new Map(
    [...rule.body.matchAll(/(--[\w-]+)\s*:\s*([^;]+)/g)].map(([, name, value]) => [name, value.trim()])
  );
}

function luminance(hex) {
  const digits = hex.slice(1);
  const full = digits.length === 3 ? [...digits].map((d) => d + d).join('') : digits.slice(0, 6);
  const [r, g, b] = [0, 2, 4].map((at) => {
    const channel = parseInt(full.slice(at, at + 2), 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrastRatio(a, b) {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (light + 0.05) / (dark + 0.05);
}

function contrastViolations(theme, tokens, pairs = CONTRAST_PAIRS) {
  const violations = [];
  for (const { fg, bg, min } of pairs) {
    const [a, b] = [tokens.get(fg), tokens.get(bg)];
    if (!/^#[0-9a-fA-F]{3,6}$/.test(a ?? '') || !/^#[0-9a-fA-F]{3,6}$/.test(b ?? '')) {
      violations.push(`${theme}: ${fg} on ${bg} needs two opaque hex tokens, got "${a}" and "${b}"`);
      continue;
    }
    const ratio = contrastRatio(a, b);
    if (ratio < min) {
      violations.push(`${theme}: ${fg} ${a} on ${bg} ${b} is ${ratio.toFixed(2)}:1, want at least ${min}:1`);
    }
  }
  return violations;
}

export async function findThemeViolations(base = '.') {
  const css = await readFile(join(base, TOKEN_FILE), 'utf8');
  const light = tokenBlock(css, ':root');
  const dark = tokenBlock(css, DARK_SELECTOR);
  const systemDark = tokenBlock(css, SYSTEM_DARK_SELECTOR);
  if (!light || !dark || !systemDark) {
    return [`${TOKEN_FILE} needs :root, ${DARK_SELECTOR} and ${SYSTEM_DARK_SELECTOR} token blocks`];
  }

  const violations = [];
  for (const [name, value] of dark) {
    if (systemDark.get(name) !== value) {
      violations.push(`${SYSTEM_DARK_SELECTOR} ${name} is "${systemDark.get(name)}", ${DARK_SELECTOR} has "${value}"`);
    }
  }
  for (const name of systemDark.keys()) {
    if (!dark.has(name)) violations.push(`${SYSTEM_DARK_SELECTOR} defines ${name}, ${DARK_SELECTOR} does not`);
  }
  for (const name of dark.keys()) {
    if (!light.has(name)) violations.push(`${DARK_SELECTOR} defines ${name}, which :root does not`);
  }
  for (const name of light.keys()) {
    if (/^--(z-|mobile-|shadow$)/.test(name)) continue;
    if (!dark.has(name)) violations.push(`:root defines ${name} with no dark value`);
  }
  violations.push(...contrastViolations('light', light), ...contrastViolations('dark', new Map([...light, ...dark])));
  violations.push(...readerThemeViolations(css));
  return violations;
}

function readerThemeViolations(css) {
  const blocks = Object.entries(READER_THEME_SELECTORS).map(([theme, selector]) => [
    theme,
    selector,
    tokenBlock(css, selector, { whole: true })
  ]);
  const missing = blocks.filter(([, , block]) => !block).map(([, selector]) => selector);
  if (missing.length > 0) return [`${TOKEN_FILE} needs reader token blocks: ${missing.join(', ')}`];

  const [, , light] = blocks[0];
  const readerTokens = [...light.keys()].filter((name) => name.startsWith('--reader-'));
  const violations = [];
  for (const [theme, selector, block] of blocks.slice(1)) {
    for (const name of readerTokens) {
      if (!block.has(name)) violations.push(`${selector} has no ${name}`);
    }
    for (const name of block.keys()) {
      if (!light.has(name)) violations.push(`${selector} defines ${name}, which ${READER_THEME_SELECTORS.light} does not`);
    }
  }
  for (const [theme, , block] of blocks) {
    violations.push(...contrastViolations(`reader ${theme}`, new Map([...light, ...block]), READER_CONTRAST_PAIRS));
  }
  return violations;
}
