import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import {
  CONTRACTS,
  declaration,
  findViolations,
  ruleFor,
  rules,
  styleSource
} from './check-style-contracts.mjs';

const SETTINGS_PAGE = 'src/features/settings/pages/SettingsPage.vue';
const READER_CSS = 'src/features/reader/styles/reader-content.css';

/** A tree that honours every contract, for a test to break one field of. */
function healthyTree() {
  return {
    [SETTINGS_PAGE]: `<template>
  <TabsContent class="settings-tab-content">{{ label }}</TabsContent>
</template>

<style scoped>
.settings-tab-content {
  display: grid;
  gap: 16px;
}

/* .settings-tab-content[hidden] belongs here, not in the comment. */
.settings-tab-content[hidden] {
  display: none;
}
</style>
`,
    [READER_CSS]: `.reader-text {
  font-family: var(--reader-font-family, Georgia, serif);
}

.reader-safe-html :deep(h1),
.reader-safe-html :deep(h2) {
  font-family: var(--reader-font-family, Georgia, serif);
}

.reader-safe-html :deep(.reader-md-inline-code) {
  font-family: 'SFMono-Regular', Consolas, monospace;
}
`
  };
}

let base = null;

/** Writes a tree and returns what each contract found in it, contract order kept. */
async function check(overrides = {}) {
  base = await mkdtemp(join(tmpdir(), 'style-contracts-'));
  const files = { ...healthyTree(), ...overrides };
  for (const [path, source] of Object.entries(files)) {
    if (source === null) {
      continue;
    }
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
  it('reports nothing when every contract is honoured', async () => {
    expect(await check()).toEqual([[], [], [], []]);
  });

  it('reports a file it cannot read rather than passing it', async () => {
    const [settings] = await check({ [SETTINGS_PAGE]: null });
    expect(settings).toEqual([`${SETTINGS_PAGE} is missing`]);
  });

  it('catches the collapse rule being dropped', async () => {
    const tree = healthyTree();
    const [settings] = await check({
      [SETTINGS_PAGE]: tree[SETTINGS_PAGE].replace(
        /\.settings-tab-content\[hidden\] \{[^}]*\}/,
        ''
      )
    });
    expect(settings).toEqual([`${SETTINGS_PAGE}: no rule selects .settings-tab-content[hidden]`]);
  });

  it('catches the collapse rule being weakened to another display', async () => {
    const tree = healthyTree();
    const [settings] = await check({
      [SETTINGS_PAGE]: tree[SETTINGS_PAGE].replace(
        '.settings-tab-content[hidden] {\n  display: none;\n}',
        '.settings-tab-content[hidden] {\n  display: grid;\n}'
      )
    });
    expect(settings).toHaveLength(1);
    expect(settings[0]).toContain('display is "grid"');
  });

  it('catches a rule that keeps the selector but drops the declaration', async () => {
    const tree = healthyTree();
    const [, readerText] = await check({
      [READER_CSS]: tree[READER_CSS].replace(
        '.reader-text {\n  font-family: var(--reader-font-family, Georgia, serif);\n}',
        '.reader-text {\n  color: #2f2a22;\n}'
      )
    });
    expect(readerText).toEqual([`${READER_CSS}: .reader-text declares no font-family`]);
  });

  it('catches body text being pinned to a fixed family', async () => {
    const tree = healthyTree();
    const [, readerText] = await check({
      [READER_CSS]: tree[READER_CSS].replace(
        '.reader-text {\n  font-family: var(--reader-font-family, Georgia, serif);\n}',
        '.reader-text {\n  font-family: Georgia, serif;\n}'
      )
    });
    expect(readerText).toHaveLength(1);
    expect(readerText[0]).toContain('font-family is "Georgia, serif"');
  });

  it('catches code reaching the reading font, even while it stays monospace', async () => {
    const tree = healthyTree();
    const [, , , code] = await check({
      [READER_CSS]: tree[READER_CSS].replace(
        "font-family: 'SFMono-Regular', Consolas, monospace;",
        'font-family: var(--reader-font-family, monospace);'
      )
    });
    expect(code).toHaveLength(1);
    expect(code[0]).toContain('must not match');
  });
});

describe('rules', () => {
  it('descends into an at-rule block instead of reading it as a selector', () => {
    expect(
      rules('@media (max-width: 720px) {\n  .reader-text {\n    font-size: 18px;\n  }\n}')
    ).toEqual([{ selector: '.reader-text', body: '\n    font-size: 18px;\n  ' }]);
  });

  it('ignores comments, so a selector named in one is not mistaken for a rule', () => {
    expect(rules('/* .reader-text { font-family: serif; } */\n.other { color: red; }')).toEqual([
      { selector: '.other', body: ' color: red; ' }
    ]);
  });
});

describe('ruleFor', () => {
  it('matches a selector anywhere in a grouped selector list', () => {
    const rule = ruleFor('.a :deep(h1),\n.a :deep(h2) {\n  font-family: serif;\n}', ':deep(h2)');
    expect(rule?.selector).toContain(':deep(h1)');
  });

  it('returns null when nothing selects it', () => {
    expect(ruleFor('.a { color: red; }', '.b')).toBeNull();
  });
});

describe('declaration', () => {
  it('reads a value and stops at the semicolon', () => {
    expect(declaration(' display: none; gap: 16px; ', 'display')).toBe('none');
  });

  it('does not match a property that merely ends with the name', () => {
    expect(declaration(' background-color: red; ', 'color')).toBeNull();
  });

  it('returns null when the property is absent', () => {
    expect(declaration(' gap: 16px; ', 'display')).toBeNull();
  });
});

describe('styleSource', () => {
  it('reads only the style blocks of a .vue file', () => {
    const source = '<template>\n  <p>{{ x }}</p>\n</template>\n<style scoped>\n.a { color: red; }\n</style>\n';
    expect(styleSource(source, 'A.vue')).not.toContain('template');
    expect(styleSource(source, 'A.vue')).toContain('.a { color: red; }');
  });

  it('reads a .css file whole', () => {
    expect(styleSource('.a { color: red; }', 'a.css')).toBe('.a { color: red; }');
  });
});

describe('CONTRACTS', () => {
  it('names a reason for every contract, which is what the failure prints', () => {
    for (const contract of CONTRACTS) {
      expect(contract.why, contract.name).toBeTruthy();
    }
  });
});
