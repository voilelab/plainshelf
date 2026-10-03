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
const MOBILE_READER = 'src/features/reader/components/MobileReaderView.vue';

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
    [MOBILE_READER]: `<template>
  <section class="mobile-reader-page"><button class="mobile-reader-tool" /></section>
</template>

<style scoped>
.mobile-reader-toolbar {
  padding: 8px;
}

.mobile-reader-tool {
  height: 44px;
  min-height: 44px;
}

.mobile-reader-page :deep(.reader-md-code) {
  touch-action: pan-x pan-y pinch-zoom;
}
</style>
`,
    [READER_CSS]: `.reader-text {
  font-family: var(--reader-font-family, Georgia, serif);
  font-size: var(--reader-font-size, 20px);
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
  return Object.fromEntries(results.map(({ contract, violations }) => [contract.name, violations]));
}

afterEach(async () => {
  if (base) await rm(base, { recursive: true, force: true });
  base = null;
});

describe('findViolations', () => {
  it('reports nothing when every contract is honoured', async () => {
    expect(Object.values(await check()).flat()).toEqual([]);
  });

  it('reports a file it cannot read rather than passing it', async () => {
    const found = await check({ [SETTINGS_PAGE]: null });
    expect(found['hidden settings panels collapse']).toEqual([`${SETTINGS_PAGE} is missing`]);
  });

  it('catches the collapse rule being dropped', async () => {
    const tree = healthyTree();
    const found = await check({
      [SETTINGS_PAGE]: tree[SETTINGS_PAGE].replace(
        /\.settings-tab-content\[hidden\] \{[^}]*\}/,
        ''
      )
    });
    expect(found['hidden settings panels collapse']).toEqual([
      `${SETTINGS_PAGE}: no rule selects .settings-tab-content[hidden]`
    ]);
  });

  it('catches the collapse rule being weakened to another display', async () => {
    const tree = healthyTree();
    const found = await check({
      [SETTINGS_PAGE]: tree[SETTINGS_PAGE].replace(
        '.settings-tab-content[hidden] {\n  display: none;\n}',
        '.settings-tab-content[hidden] {\n  display: grid;\n}'
      )
    });
    expect(found['hidden settings panels collapse']).toHaveLength(1);
    expect(found['hidden settings panels collapse'][0]).toContain('display is "grid"');
  });

  it('catches a rule that keeps the selector but drops the declaration', async () => {
    const tree = healthyTree();
    const found = await check({
      [READER_CSS]: tree[READER_CSS].replace(
        'font-family: var(--reader-font-family, Georgia, serif);',
        'color: #2f2a22;'
      )
    });
    expect(found['body text follows the reading font']).toEqual([
      `${READER_CSS}: .reader-text declares no font-family`
    ]);
  });

  it('catches body text being pinned to a fixed family', async () => {
    const tree = healthyTree();
    const found = await check({
      [READER_CSS]: tree[READER_CSS].replace(
        'font-family: var(--reader-font-family, Georgia, serif);',
        'font-family: Georgia, serif;'
      )
    });
    expect(found['body text follows the reading font']).toHaveLength(1);
    expect(found['body text follows the reading font'][0]).toContain('font-family is "Georgia, serif"');
  });

  it('catches code reaching the reading font, even while it stays monospace', async () => {
    const tree = healthyTree();
    const found = await check({
      [READER_CSS]: tree[READER_CSS].replace(
        "font-family: 'SFMono-Regular', Consolas, monospace;",
        'font-family: var(--reader-font-family, monospace);'
      )
    });
    expect(found['code keeps a monospace stack']).toHaveLength(1);
    expect(found['code keeps a monospace stack'][0]).toContain('must not match');
  });
});

describe('the mobile reader contracts', () => {
  it('catches a code block that stops panning on its own axis', async () => {
    const tree = healthyTree();
    const found = await check({
      [MOBILE_READER]: tree[MOBILE_READER].replace(
        'touch-action: pan-x pan-y pinch-zoom;',
        'touch-action: pan-y pinch-zoom;'
      )
    });
    expect(found['a code block pans on its own axis']).toHaveLength(1);
  });

  // .mobile-reader-toolbar is declared first and contains the tool's own name,
  // so a substring match would read the toolbar's padding as the tool's floor.
  it('reads the tool floor off the tool, not off the toolbar', async () => {
    const tree = healthyTree();
    const found = await check({
      [MOBILE_READER]: tree[MOBILE_READER].replace('min-height: 44px;', 'min-height: 30px;')
    });
    expect(found['toolbar controls stay thumb-sized']).toHaveLength(1);
    expect(found['toolbar controls stay thumb-sized'][0]).toContain('"30px"');
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

  // .mobile-reader-tool and .mobile-reader-toolbar are two rules with two jobs;
  // a substring match answers a contract about one with the other's body.
  it('does not answer a selector with a longer name that contains it', () => {
    const css = '.tool-bar {\n  padding: 8px;\n}\n\n.tool {\n  min-height: 44px;\n}\n';
    expect(ruleFor(css, '.tool')?.body).toContain('min-height');
  });

  it('still finds a selector nested inside a longer one', () => {
    const rule = ruleFor('.a :deep(h1) {\n  font-family: serif;\n}', ':deep(h1)');
    expect(rule?.selector).toBe('.a :deep(h1)');
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
