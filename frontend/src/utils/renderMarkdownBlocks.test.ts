// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import {
  READER_BLOCK_CHAR_BUDGET,
  READER_BLOCK_LINE_BUDGET,
  renderMarkdownBlocks
} from './renderMarkdownBlocks';
import { sanitizeHtml } from './safeHtml';

/** A one-line paragraph of the given character length, tagged for lookup. */
function paragraph(tag: string, length: number): string {
  return tag + 'x'.repeat(Math.max(0, length - tag.length));
}

describe('renderMarkdownBlocks', () => {
  it('renders Markdown and raw HTML together', () => {
    const blocks = renderMarkdownBlocks([
      '# Title',
      '',
      '<details><summary>Info</summary>',
      '',
      '**Bold** body',
      '</details>'
    ].join('\n'));

    expect(blocks).toHaveLength(1);
    expect(blocks[0]).toMatchObject({ type: 'html' });
    expect((blocks[0] as { html: string }).html).toContain('<h2 class="reader-md-h1">Title</h2>');
    expect((blocks[0] as { html: string }).html).toContain('<details><summary>Info</summary>');
    expect((blocks[0] as { html: string }).html).toContain('<strong>Bold</strong> body');
  });

  it('treats standalone plot markers as a transparent Markdown container', () => {
    const [block] = renderMarkdownBlocks([
      '<plot>',
      'First paragraph.',
      '',
      'Second paragraph.',
      '',
      '<p style="color: purple">Thought</p>',
      '</plot>'
    ].join('\n'));

    expect(block.type).toBe('html');
    const html = (block as { html: string }).html;
    expect(html).not.toContain('<plot');
    expect(html).toContain('<p class="reader-text-block">First paragraph.</p>');
    expect(html).toContain('<p class="reader-text-block">Second paragraph.</p>');
    expect(html).toContain('<p style="color: purple">Thought</p>');
  });

  it('keeps plot markers literal inside fenced code', () => {
    const [block] = renderMarkdownBlocks('```html\n<plot>\nStory\n</plot>\n```');
    const html = (block as { html: string }).html;
    expect(html).toContain('&lt;plot&gt;');
    expect(html).toContain('&lt;/plot&gt;');
  });

  it('keeps fenced HTML escaped and non-executable', () => {
    const blocks = renderMarkdownBlocks('```html\n<script>alert(1)</script>\n```');
    expect(blocks).toEqual([{
      type: 'html',
      html: '<pre class="reader-md-code"><code>&lt;script&gt;alert(1)&lt;/script&gt;\n</code></pre>\n',
      images: []
    }]);
  });

  it('keeps only valid standalone local images as component slots', () => {
    const blocks = renderMarkdownBlocks([
      'before',
      '![local](assets/cover.png)',
      '![external](https://example.com/x.png)',
      'after'
    ].join('\n'));

    expect(blocks).toHaveLength(1);
    expect(blocks[0].images).toMatchObject([{ name: 'cover.png', alt: 'local' }]);
    expect(blocks[0].html).toContain('class="reader-asset-slot"');
    expect(blocks[0].html).toContain('![external](https://example.com/x.png)');
    expect(blocks[0].html).not.toContain('<img');
  });

  it('preserves an HTML container around a local image and following content', () => {
    const [block] = renderMarkdownBlocks([
      '<details><summary>Map</summary>',
      '',
      '![floor plan](assets/map.png)',
      '',
      'After the map.',
      '</details>'
    ].join('\n'));

    expect(block.images).toMatchObject([{ name: 'map.png', alt: 'floor plan' }]);
    const clean = sanitizeHtml(block.html, 'reader');
    const template = document.createElement('template');
    template.innerHTML = clean;
    const details = template.content.querySelector('details');
    expect(details?.querySelector('.reader-asset-slot')).not.toBeNull();
    expect(details?.textContent).toContain('After the map.');
  });

  it('keeps an indented list image inside the list item', () => {
    const [block] = renderMarkdownBlocks('- Floor\n  ![map](assets/map.png)');

    expect(block.images).toMatchObject([{ name: 'map.png', alt: 'map' }]);
    const clean = sanitizeHtml(block.html, 'reader');
    const template = document.createElement('template');
    template.innerHTML = clean;
    expect(template.content.querySelector('li .reader-asset-slot')).not.toBeNull();
  });

  it('keeps an image-looking line literal inside a raw pre block', () => {
    const [block] = renderMarkdownBlocks('<pre>\n![map](assets/map.png)\n</pre>');

    expect(block.images).toEqual([]);
    expect(block.html).toContain('![map](assets/map.png)');
    expect(block.html).not.toContain('reader-asset-slot');
  });

  it('does not extract an image from a code fence', () => {
    const blocks = renderMarkdownBlocks('```md\n![local](assets/cover.png)\n```');
    expect(blocks).toHaveLength(1);
    expect(blocks[0].type).toBe('html');
    expect(blocks[0].html).toContain('![local](assets/cover.png)');
    expect(blocks[0].images).toEqual([]);
  });

  // Every one of these classes is in the sanitizer's allowlist and carries the
  // reader's styling. A wrong tag or class name survives sanitization as
  // unstyled markup, so nothing downstream reports it.
  it('maps heading levels onto the reader\'s own tags and classes', () => {
    const [block] = renderMarkdownBlocks('# H1\n\n## H2\n\n### H3\n\n#### H4\n\n###### H6');
    expect(block.html).toContain('<h2 class="reader-md-h1">H1</h2>');
    expect(block.html).toContain('<h3 class="reader-md-h2">H2</h3>');
    expect(block.html).toContain('<h4 class="reader-md-h3">H3</h4>');
    expect(block.html).toContain('<h4 class="reader-md-h3">H4</h4>');
    expect(block.html).toContain('<h4 class="reader-md-h3">H6</h4>');
    expect(block.html).not.toContain('<h1');
  });

  it('gives quotes, lists, rules, and inline code their reader classes', () => {
    const [quote] = renderMarkdownBlocks('> Quoted');
    expect(quote.html).toContain('<blockquote class="reader-text-block reader-text-quote">');

    const [lists] = renderMarkdownBlocks('- one\n\n1. one');
    expect(lists.html).toContain('<ul class="reader-md-list">');
    expect(lists.html).toContain('<ol class="reader-md-list">');

    const [rule] = renderMarkdownBlocks('a\n\n---\n\nb');
    expect(rule.html).toContain('<hr class="reader-md-hr">');

    const [inline] = renderMarkdownBlocks('a `x<y>` b');
    expect(inline.html).toContain('<code class="reader-md-inline-code">x&lt;y&gt;</code>');
  });

  // An illustration is a block of its own. Wrapping it in the paragraph the
  // Markdown source implies would give it reading-text spacing and styling.
  it('emits a standalone illustration without a paragraph around it', () => {
    const [block] = renderMarkdownBlocks('![local](assets/cover.png)');
    expect(block.html).toBe(`<span class="reader-asset-slot" title="${block.images[0].token}"></span>`);
  });

  it('keeps the paragraph when an illustration shares it with prose', () => {
    const [block] = renderMarkdownBlocks('Floor\n![map](assets/map.png)');
    expect(block.html).toBe(
      `<p class="reader-text-block">Floor<br>\n<span class="reader-asset-slot" title="${block.images[0].token}"></span></p>\n`
    );
  });

  // The slot replaces the image line in place, so an image indented into a
  // list item has to stay indented or it leaves the item it illustrates.
  it('keeps an illustration inside the list item it was indented under', () => {
    const [block] = renderMarkdownBlocks('- Floor\n\n  ![map](assets/map.png)\n');
    const clean = sanitizeHtml(block.html, 'reader');
    const template = document.createElement('template');
    template.innerHTML = clean;
    expect(template.content.querySelector('li .reader-asset-slot')).not.toBeNull();
  });

  // A marker line is removed only when it is the whole line, so prose that
  // mentions the tag keeps it, and the reader never depends on a bare marker
  // being written without attributes or padding.
  it('removes a plot marker line whatever it carries around it', () => {
    for (const source of [
      '<plot class="x">\nStory\n</plot>',
      '  <plot>  \nStory\n  </plot>  ',
      '</plot>\nStory'
    ]) {
      const [block] = renderMarkdownBlocks(source);
      expect(block.html, source).toBe('<p class="reader-text-block">Story</p>\n');
    }
  });

  it('keeps a plot marker that shares its line with prose', () => {
    const [block] = renderMarkdownBlocks('text <plot>\nStory');
    expect(block.html).toContain('text <plot>');
  });

  it('produces no block for a source that is only whitespace', () => {
    expect(renderMarkdownBlocks('')).toEqual([]);
    expect(renderMarkdownBlocks('   \n\n  ')).toEqual([]);
  });

  it('produces no block for a source that is only container markers', () => {
    expect(renderMarkdownBlocks('<plot>\n</plot>')).toEqual([]);
  });

  // Only a whole line is a marker. Cutting the line at the closing bracket
  // would take the sentence after it with it.
  it('keeps the text that follows a plot marker on the same line', () => {
    const [block] = renderMarkdownBlocks('<plot>Story');
    expect(block.html).toContain('Story');
  });

  // The paragraph is suppressed only for an illustration standing alone. An
  // image target the reader will not load is prose, and a caption line shares
  // the paragraph with the slot; dropping the paragraph in either case would
  // run that text into its neighbours.
  it('keeps the paragraph around an image the reader does not load', () => {
    const [block] = renderMarkdownBlocks('![external](https://example.com/x.png)');
    expect(block.html)
      .toBe('<p class="reader-text-block">![external](https://example.com/x.png)</p>\n');
  });

  it('keeps the paragraph when a caption follows the illustration', () => {
    const [block] = renderMarkdownBlocks('- Floor\n\n  ![map](assets/map.png)\n  caption text');
    expect(block.html).toContain(
      `<p class="reader-text-block"><span class="reader-asset-slot" title="${block.images[0].token}"></span><br>\ncaption text</p>`
    );
  });

  // Two documents on screen at once must not answer to each other's slot
  // tokens, so the prefix has to change on every render.
  it('gives each rendered document its own asset token prefix', () => {
    const first = renderMarkdownBlocks('![a](assets/cover.png)')[0].images[0].token;
    const second = renderMarkdownBlocks('![a](assets/cover.png)')[0].images[0].token;
    expect(first).not.toBe(second);
  });

  // The reader shows what the author wrote. Typographic substitution would
  // rewrite quotes, dashes, and (c) behind their back.
  it('leaves the author\'s punctuation exactly as written', () => {
    const [block] = renderMarkdownBlocks('"quoted" -- dash (c) 1/2');
    expect(block.html)
      .toBe('<p class="reader-text-block">&quot;quoted&quot; -- dash (c) 1/2</p>\n');
  });

  it('leaves Markdown links literal rather than creating navigation', () => {
    const [block] = renderMarkdownBlocks('[outside](https://example.com)');
    expect((block as { html: string }).html).toContain('[outside](https://example.com)');
    expect((block as { html: string }).html).not.toContain('<a');
  });
});

describe('renderMarkdownBlocks section chunking', () => {
  const oversized = READER_BLOCK_CHAR_BUDGET + 500;

  it('keeps a small section as a single block', () => {
    const blocks = renderMarkdownBlocks('One paragraph.\n\nTwo paragraphs.');
    expect(blocks).toHaveLength(1);
  });

  it('cuts a large section at paragraph boundaries once the char budget is met', () => {
    const source = [
      paragraph('P1_', READER_BLOCK_CHAR_BUDGET),
      paragraph('P2_', READER_BLOCK_CHAR_BUDGET),
      paragraph('P3_', READER_BLOCK_CHAR_BUDGET)
    ].join('\n\n');
    const blocks = renderMarkdownBlocks(source);

    expect(blocks.length).toBeGreaterThan(1);
    // Every paragraph lands whole in exactly one block: no tag is split across
    // blocks, and each block ends at a paragraph boundary (closes its <p>).
    for (const tag of ['P1_', 'P2_', 'P3_']) {
      const owners = blocks.filter((block) => block.html.includes(tag));
      expect(owners).toHaveLength(1);
    }
    for (const block of blocks) {
      expect(block.html.trim().endsWith('</p>')).toBe(true);
    }
  });

  it('cuts on the line budget for many short paragraphs', () => {
    const source = Array.from({ length: READER_BLOCK_LINE_BUDGET * 2 }, (_unused, index) => `L${index}`)
      .join('\n\n');
    const blocks = renderMarkdownBlocks(source);
    expect(blocks.length).toBeGreaterThan(1);
    // First and last markers survive and stay in order across the split.
    expect(blocks[0].html).toContain('L0');
    expect(blocks[blocks.length - 1].html).toContain(`L${READER_BLOCK_LINE_BUDGET * 2 - 1}`);
  });

  it('normalizes and splits a CRLF section without dropping content', () => {
    const source = [
      paragraph('C1_', READER_BLOCK_CHAR_BUDGET),
      paragraph('C2_', READER_BLOCK_CHAR_BUDGET)
    ].join('\r\n\r\n');
    const blocks = renderMarkdownBlocks(source);

    expect(blocks.length).toBeGreaterThan(1);
    expect(blocks.some((block) => block.html.includes('C1_'))).toBe(true);
    expect(blocks.some((block) => block.html.includes('C2_'))).toBe(true);
    for (const block of blocks) {
      expect(block.html).not.toContain('\r');
    }
  });

  it('keeps a single over-budget paragraph whole rather than cutting mid-line', () => {
    const blocks = renderMarkdownBlocks(paragraph('LONG_', oversized));
    expect(blocks).toHaveLength(1);
    expect(blocks[0].html).toContain('LONG_');
  });

  it('never cuts inside an over-budget fenced code block', () => {
    const code = Array.from({ length: 120 }, (_unused, index) => `line ${index} ${'z'.repeat(40)}`)
      .join('\n');
    const source = `intro paragraph\n\n\`\`\`\n${code}\n\`\`\`\n\ntail paragraph`;
    const blocks = renderMarkdownBlocks(source);

    const fenceBlocks = blocks.filter((block) => block.html.includes('reader-md-code'));
    // The whole fence renders in exactly one <pre>: not split into two code blocks.
    expect(fenceBlocks).toHaveLength(1);
    expect((fenceBlocks[0].html.match(/reader-md-code/g) ?? [])).toHaveLength(1);
    expect(fenceBlocks[0].html).toContain('line 0 ');
    expect(fenceBlocks[0].html).toContain('line 119 ');
  });

  it('never splits an over-budget ordered list into renumbered halves', () => {
    const items = Array.from(
      { length: 40 },
      (_unused, index) => `${index + 1}. item ${index} ${'y'.repeat(60)}`
    ).join('\n');
    const source = `${items}\n\ntrailing paragraph`;
    const blocks = renderMarkdownBlocks(source);

    const listBlocks = blocks.filter((block) => block.html.includes('<ol'));
    expect(listBlocks).toHaveLength(1);
    // A single <ol> means list-item numbering is never restarted mid-list.
    expect((listBlocks[0].html.match(/<ol/g) ?? [])).toHaveLength(1);
    expect(listBlocks[0].html).toContain('item 0 ');
    expect(listBlocks[0].html).toContain('item 39 ');
  });

  it('attributes each illustration only to the block that renders it', () => {
    const source = [
      '![first](assets/a.png)',
      paragraph('MID_', oversized),
      '![second](assets/b.png)'
    ].join('\n\n');
    const blocks = renderMarkdownBlocks(source);

    expect(blocks.length).toBeGreaterThan(1);
    const withA = blocks.filter((block) => block.images.some((image) => image.name === 'a.png'));
    const withB = blocks.filter((block) => block.images.some((image) => image.name === 'b.png'));
    expect(withA).toHaveLength(1);
    expect(withB).toHaveLength(1);
    // The two illustrations land in different blocks, and neither block carries
    // the other's asset token.
    expect(withA[0]).not.toBe(withB[0]);
    expect(withA[0].images).toHaveLength(1);
    expect(withB[0].images).toHaveLength(1);
  });
});
