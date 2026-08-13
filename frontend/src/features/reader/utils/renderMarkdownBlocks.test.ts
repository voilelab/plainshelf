import { describe, expect, it } from 'vitest';
import { renderMarkdownBlocks } from './renderMarkdownBlocks';

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
      html: '<pre class="reader-md-code"><code>&lt;script&gt;alert(1)&lt;/script&gt;\n</code></pre>\n'
    }]);
  });

  it('keeps only valid standalone local images as image blocks', () => {
    const blocks = renderMarkdownBlocks([
      'before',
      '![local](assets/cover.png)',
      '![external](https://example.com/x.png)',
      'after'
    ].join('\n'));

    expect(blocks.map((block) => block.type)).toEqual(['html', 'image', 'html']);
    expect(blocks[1]).toEqual({ type: 'image', name: 'cover.png', alt: 'local' });
    expect((blocks[2] as { html: string }).html).toContain('![external](https://example.com/x.png)');
    expect((blocks[2] as { html: string }).html).not.toContain('<img');
  });

  it('does not extract an image from a code fence', () => {
    const blocks = renderMarkdownBlocks('```md\n![local](assets/cover.png)\n```');
    expect(blocks).toHaveLength(1);
    expect(blocks[0].type).toBe('html');
    expect((blocks[0] as { html: string }).html).toContain('![local](assets/cover.png)');
  });

  it('leaves Markdown links literal rather than creating navigation', () => {
    const [block] = renderMarkdownBlocks('[outside](https://example.com)');
    expect((block as { html: string }).html).toContain('[outside](https://example.com)');
    expect((block as { html: string }).html).not.toContain('<a');
  });
});
