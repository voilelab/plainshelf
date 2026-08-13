// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { renderMarkdownBlocks } from './renderMarkdownBlocks';
import { sanitizeReaderHtml } from './sanitizeReaderHtml';

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
    const clean = sanitizeReaderHtml(block.html);
    const template = document.createElement('template');
    template.innerHTML = clean;
    const details = template.content.querySelector('details');
    expect(details?.querySelector('.reader-asset-slot')).not.toBeNull();
    expect(details?.textContent).toContain('After the map.');
  });

  it('keeps an indented list image inside the list item', () => {
    const [block] = renderMarkdownBlocks('- Floor\n  ![map](assets/map.png)');

    expect(block.images).toMatchObject([{ name: 'map.png', alt: 'map' }]);
    const clean = sanitizeReaderHtml(block.html);
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

  it('leaves Markdown links literal rather than creating navigation', () => {
    const [block] = renderMarkdownBlocks('[outside](https://example.com)');
    expect((block as { html: string }).html).toContain('[outside](https://example.com)');
    expect((block as { html: string }).html).not.toContain('<a');
  });
});
