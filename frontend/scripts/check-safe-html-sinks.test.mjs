import { readFile, readdir } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { parse } from 'vue/compiler-sfc';

function htmlDirectives(source) {
  const { descriptor, errors } = parse(source);
  if (errors.length) throw errors[0];
  const found = [];
  function visit(node) {
    for (const prop of node.props ?? []) {
      if (prop.type === 7 && prop.name === 'html') found.push(prop);
    }
    for (const child of node.children ?? []) visit(child);
  }
  if (descriptor.template) visit(descriptor.template.ast);
  return found;
}

describe('sanitized HTML boundary', () => {
  it('recognizes v-html directives while ignoring comments and text', () => {
    expect(htmlDirectives('<template><!-- v-html="raw" --><p>v-html</p></template>')).toHaveLength(0);
    expect(htmlDirectives('<template><section><div v-html="raw" /></section></template>')).toHaveLength(1);
  });

  it('permits v-html only in SafeHtml with its sanitizedHtml binding', async () => {
    const root = fileURLToPath(new URL('../src/', import.meta.url));
    const sinks = [];
    for (const file of await readdir(root, { recursive: true })) {
      if (!file.endsWith('.vue')) continue;
      for (const directive of htmlDirectives(await readFile(join(root, file), 'utf8'))) {
        sinks.push([relative(root, join(root, file)).replaceAll('\\', '/'), directive.exp?.content]);
      }
    }
    expect(sinks).toEqual([['components/SafeHtml.vue', 'sanitizedHtml']]);
  });
});
