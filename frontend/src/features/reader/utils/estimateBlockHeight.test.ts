import { describe, expect, it } from 'vitest';
import {
  estimateMarkdownBlockHeight,
  estimateTextBlockHeight
} from './estimateBlockHeight';
import type { ReaderMarkdownAsset } from '@/utils/renderMarkdownBlocks';

function asset(token: string): ReaderMarkdownAsset {
  return { token, name: 'x.png', alt: '', src: 'assets/x.png' } as unknown as ReaderMarkdownAsset;
}

describe('estimateMarkdownBlockHeight', () => {
  it('reserves real space for an asset-only block instead of one empty line', () => {
    const image = estimateMarkdownBlockHeight({
      type: 'html',
      html: '<span class="reader-asset-slot" title="t"></span>',
      images: [asset('t')]
    });
    const emptyish = estimateMarkdownBlockHeight({ type: 'html', html: '<p></p>', images: [] });
    expect(image).toBeGreaterThan(300);
    expect(image).toBeGreaterThan(emptyish * 3);
  });

  it('does not treat a long single code line as wrapped prose', () => {
    const longLine = 'a'.repeat(400);
    const code = estimateMarkdownBlockHeight({
      type: 'html',
      html: `<pre class="reader-md-code"><code>${longLine}\n</code></pre>`,
      images: []
    });
    const prose = estimateMarkdownBlockHeight({
      type: 'html',
      html: `<p class="reader-text-block">${longLine}</p>`,
      images: []
    });
    // The code line does not wrap, so it must estimate far shorter than the same
    // number of characters flowed as prose.
    expect(code).toBeLessThan(prose);
  });

  it('counts each code line as its own row', () => {
    const oneLine = estimateMarkdownBlockHeight({
      type: 'html',
      html: '<pre class="reader-md-code"><code>a\n</code></pre>',
      images: []
    });
    const tenLines = estimateMarkdownBlockHeight({
      type: 'html',
      html: `<pre class="reader-md-code"><code>${'a\n'.repeat(10)}</code></pre>`,
      images: []
    });
    expect(tenLines).toBeGreaterThan(oneLine);
  });

  it('grows with prose length', () => {
    const short = estimateMarkdownBlockHeight({
      type: 'html',
      html: '<p class="reader-text-block">short</p>',
      images: []
    });
    const long = estimateMarkdownBlockHeight({
      type: 'html',
      html: `<p class="reader-text-block">${'word '.repeat(400)}</p>`,
      images: []
    });
    expect(long).toBeGreaterThan(short);
  });
});

describe('estimateTextBlockHeight', () => {
  it('grows with text length and never collapses to zero', () => {
    const short = estimateTextBlockHeight({ type: 'paragraph', text: 'hi' });
    const long = estimateTextBlockHeight({ type: 'paragraph', text: 'x'.repeat(2000) });
    expect(short).toBeGreaterThan(0);
    expect(long).toBeGreaterThan(short);
  });
});
