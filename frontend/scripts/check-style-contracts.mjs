/**
 * Fails when a stylesheet stops honouring a rule the rest of the app depends on
 * but no test can observe.
 *
 * These four checks used to be `it(...)` blocks that read their own source file
 * with `readFileSync` and matched it with a regular expression. That is a lint
 * wearing a test's clothes: jsdom resolves no layout and applies no stylesheet,
 * so a unit test cannot answer "does this panel take a grid row" or "does code
 * keep a monospace stack" — and a test whose name promises a layout outcome
 * while its assertion proves a string is present in a file is worse than no
 * test, because it reads as coverage. Moving them here keeps exactly what they
 * checked and stops them claiming to be something else.
 *
 * A real answer needs a layout engine, which means an end-to-end case; that is
 * a budgeted slot (docs/development/testing-levels.md) and none of these earns
 * one. What they are worth is a cheap refusal to delete the rule by accident,
 * which is what this script is.
 */

import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

/**
 * Each contract names one declaration that has to survive, the file it lives
 * in, and why. `must` is the pattern the value has to match; `mustNot` is the
 * one it may not.
 */
export const CONTRACTS = [
  {
    name: 'hidden settings panels collapse',
    file: 'src/features/settings/pages/SettingsPage.vue',
    selector: '.settings-tab-content[hidden]',
    property: 'display',
    must: /\bnone\b/,
    why:
      'Reka keeps every tab panel in the DOM and only sets `hidden`. The author rule on ' +
      '.settings-tab-content outranks the user-agent `[hidden]` default, so without this pairing ' +
      'each inactive panel stays a grid item — its gap shifts the visible panel, and with ' +
      'unmount-on-hide off it renders its retained content in full.'
  },
  {
    name: 'body text follows the reading font',
    file: 'src/features/reader/styles/reader-content.css',
    selector: '.reader-text',
    property: 'font-family',
    must: /var\(--reader-font-family/,
    why: 'ReaderView sets --reader-font-family from getReaderFontFamily; body text has to read it.'
  },
  {
    name: 'headings follow the reading font',
    file: 'src/features/reader/styles/reader-content.css',
    selector: ':deep(h1)',
    property: 'font-family',
    must: /var\(--reader-font-family/,
    why: 'A heading on a different family than its body text is the bug this pins.'
  },
  {
    name: 'body text follows the reading font size',
    file: 'src/features/reader/styles/reader-content.css',
    selector: '.reader-text',
    property: 'font-size',
    must: /var\(--reader-font-size/,
    why:
      'The reader sets --reader-font-size from the device default and the font-size controls; ' +
      'a fixed size here would leave both doing nothing.'
  },
  {
    name: 'a code block pans on its own axis',
    file: 'src/features/reader/components/MobileReaderView.vue',
    selector: ':deep(.reader-md-code)',
    property: 'touch-action',
    must: /\bpan-x\b/,
    why:
      'A wide code block scrolls sideways inside the page. Without pan-x the browser hands the ' +
      'horizontal drag to the reader, which turns the chapter instead of scrolling the code.'
  },
  {
    name: 'toolbar controls stay thumb-sized',
    file: 'src/features/reader/components/MobileReaderView.vue',
    selector: '.mobile-reader-tool',
    property: 'min-height',
    must: /\b44px\b/,
    why: 'Five controls share a 320px-wide toolbar; the floor is what stops them being squeezed under a thumb.'
  },
  {
    name: 'code keeps a monospace stack',
    file: 'src/features/reader/styles/reader-content.css',
    selector: '.reader-md-inline-code',
    property: 'font-family',
    must: /\bmonospace\b/,
    mustNot: /--reader-font-family/,
    why: 'A proportional reading font mangles indentation, so code must not reach the variable.'
  }
];

/** Returns the CSS of a file: a .vue file's <style> blocks, or the whole file. */
export function styleSource(source, file) {
  if (!file.endsWith('.vue')) {
    return source;
  }
  return [...source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((match) => match[1]).join('\n');
}

/**
 * Returns every leaf rule as `{ selector, body }`, descending into at-rule
 * blocks rather than mis-reading them. A flat regular expression would take
 * `@media (…) {` as a selector and swallow the first rule inside it, which is
 * the kind of silent miss a lint cannot afford.
 */
export function rules(css) {
  const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const found = [];
  let head = '';

  for (let i = 0; i < withoutComments.length; i += 1) {
    const char = withoutComments[i];
    if (char === '{') {
      const body = blockAt(withoutComments, i);
      if (body.text.includes('{')) {
        // An at-rule block: its contents are rules of their own.
        found.push(...rules(body.text));
      } else {
        found.push({ selector: head.trim(), body: body.text });
      }
      i = body.end;
      head = '';
      continue;
    }
    if (char === '}') {
      head = '';
      continue;
    }
    head += char;
  }

  return found;
}

/** Returns the text between the brace at `open` and its match, plus that index. */
function blockAt(css, open) {
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === '{') depth += 1;
    if (css[i] === '}') {
      depth -= 1;
      if (depth === 0) {
        return { text: css.slice(open + 1, i), end: i };
      }
    }
  }
  return { text: css.slice(open + 1), end: css.length };
}

/**
 * Returns the first rule whose selector list names `selector`, or null. The
 * match is on a whole name rather than a substring: `.mobile-reader-tool` must
 * not be answered by `.mobile-reader-toolbar`, which is a different rule with a
 * different job. A named selector may still sit inside a longer one
 * (`:deep(h1)` within `.reader-safe-html :deep(h1)`), which is why this is not
 * simply an equality test.
 */
export function ruleFor(css, selector) {
  return rules(css).find((rule) => rule.selector.split(',').some((part) => names(part, selector))) ?? null;
}

/** True when `selector` appears in `part` bounded by something that is not part of a name. */
function names(part, selector) {
  const isNameChar = (char) => char !== undefined && /[A-Za-z0-9_-]/.test(char);
  let from = 0;
  for (;;) {
    const at = part.indexOf(selector, from);
    if (at < 0) {
      return false;
    }
    if (!isNameChar(part[at - 1]) && !isNameChar(part[at + selector.length])) {
      return true;
    }
    from = at + 1;
  }
}

/** Returns the declared value of `property`, or null when it is not declared. */
export function declaration(body, property) {
  const escaped = property.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = body.match(new RegExp(`(?:^|;)\\s*${escaped}\\s*:([^;]*)`));
  return match ? match[1].trim() : null;
}

/** Checks every contract against `base` and returns one entry per contract. */
export async function findViolations(base = '.') {
  const results = [];

  for (const contract of CONTRACTS) {
    const violations = [];
    let source = null;
    try {
      source = await readFile(join(base, contract.file), 'utf8');
    } catch {
      violations.push(`${contract.file} is missing`);
    }

    if (source !== null) {
      const rule = ruleFor(styleSource(source, contract.file), contract.selector);
      const value = rule && declaration(rule.body, contract.property);

      if (rule === null) {
        violations.push(`${contract.file}: no rule selects ${contract.selector}`);
      } else if (value === null) {
        violations.push(`${contract.file}: ${contract.selector} declares no ${contract.property}`);
      } else {
        if (!contract.must.test(value)) {
          violations.push(
            `${contract.file}: ${contract.selector} ${contract.property} is "${value}", want ${contract.must}`
          );
        }
        if (contract.mustNot?.test(value)) {
          violations.push(
            `${contract.file}: ${contract.selector} ${contract.property} is "${value}", which must not match ${contract.mustNot}`
          );
        }
      }
    }

    results.push({ contract, violations });
  }

  return results;
}

async function main() {
  const results = await findViolations();
  let failed = false;

  for (const { contract, violations } of results) {
    if (violations.length === 0) {
      continue;
    }

    failed = true;
    console.error(`Style contract broken: ${contract.name}\n`);
    for (const violation of violations) {
      console.error(`  ${violation}`);
    }
    console.error(`\n${contract.why}\n`);
  }

  if (failed) {
    process.exit(1);
  }

  console.log(`Style contracts OK (${CONTRACTS.length} contracts).`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
