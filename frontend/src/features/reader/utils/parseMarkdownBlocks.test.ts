import { describe, expect, it } from 'vitest';

import {
  assetNameFromSrc,
  parseMarkdownBlocks,
  referencedAssetNames,
  type MarkdownBlock
} from './parseMarkdownBlocks';

function firstBlock(text: string): MarkdownBlock {
  const blocks = parseMarkdownBlocks(text);
  expect(blocks).toHaveLength(1);
  return blocks[0];
}

describe('assetNameFromSrc', () => {
  it('accepts a single file inside assets/', () => {
    expect(assetNameFromSrc('assets/img-0001.png')).toBe('img-0001.png');
    expect(assetNameFromSrc('assets/A Map.JPEG')).toBe('A Map.JPEG');
  });

  it('refuses anything that is not a flat assets/ file', () => {
    for (const src of [
      'img-0001.png',
      '../assets/img-0001.png',
      'assets/../source.txt',
      'assets/sub/img-0001.png',
      'assets/sub\\img-0001.png',
      '/assets/img-0001.png',
      'assets/',
      'assets/.hidden.png'
    ]) {
      expect(assetNameFromSrc(src), src).toBeNull();
    }
  });

  // A book's text must not be able to make the reader fetch from the network.
  it('refuses external and inline targets', () => {
    for (const src of [
      'http://example.com/x.png',
      'https://example.com/x.png',
      'data:image/png;base64,AAAA',
      '//example.com/x.png'
    ]) {
      expect(assetNameFromSrc(src), src).toBeNull();
    }
  });

  it('refuses unsupported extensions', () => {
    for (const src of ['assets/source.txt', 'assets/img-0001', 'assets/diagram.svg']) {
      expect(assetNameFromSrc(src), src).toBeNull();
    }
  });

  // A file named "A Map.JPEG" is ordinary on a shelf edited by hand, so every
  // spelling that names it has to arrive at the same file name.
  it('accepts every spelling of a name containing a space', () => {
    for (const src of ['assets/A Map.JPEG', 'assets/A%20Map.JPEG', '<assets/A Map.JPEG>']) {
      expect(assetNameFromSrc(src), src).toBe('A Map.JPEG');
    }
  });

  // Decoding runs before validation, so an escaped separator must not slip a
  // path segment past the checks.
  it('validates after decoding, not before', () => {
    for (const src of [
      'assets/%2e%2e%2fsource.txt',
      'assets/sub%2Fimg-0001.png',
      '%2e%2e%2fassets%2fimg-0001.png'
    ]) {
      expect(assetNameFromSrc(src), src).toBeNull();
    }
  });

  it('leaves a malformed percent escape to the other checks', () => {
    expect(assetNameFromSrc('assets/100%.png')).toBe('100%.png');
    expect(assetNameFromSrc('assets/100%.txt')).toBeNull();
  });
});

describe('parseMarkdownBlocks line endings', () => {
  it('renders headings and fenced code in CRLF sources', () => {
    expect(parseMarkdownBlocks('## Part 1\r\n\r\nBody\r\n\r\n```txt\r\ncode\r\n```\r\n')).toEqual([
      { type: 'heading', level: 2, segments: [{ text: 'Part 1' }] },
      { type: 'paragraph', segments: [{ text: 'Body' }] },
      { type: 'code', text: 'code' }
    ]);
  });
});

describe('parseMarkdownBlocks images', () => {
  it('turns a line that is only an image into an image block', () => {
    expect(firstBlock('![A map](assets/img-0001.png)')).toEqual({
      type: 'image',
      name: 'img-0001.png',
      alt: 'A map'
    });
  });

  it('keeps an empty alt', () => {
    expect(firstBlock('![](assets/img-0001.png)')).toEqual({
      type: 'image',
      name: 'img-0001.png',
      alt: ''
    });
  });

  it('separates an image from the paragraphs around it', () => {
    const blocks = parseMarkdownBlocks('before\n![A map](assets/img-0001.png)\nafter');

    expect(blocks.map((block) => block.type)).toEqual(['paragraph', 'image', 'paragraph']);
  });

  // Inline images are deliberately not supported: an illustration is a block,
  // and the line must stay readable rather than silently losing content.
  it('leaves an image inside a sentence as text', () => {
    const block = firstBlock('see ![A map](assets/img-0001.png) here');

    expect(block.type).toBe('paragraph');
    expect(block).toMatchObject({
      segments: [{ text: 'see ![A map](assets/img-0001.png) here' }]
    });
  });

  // An unloadable target must remain visible instead of disappearing.
  it('falls back to a paragraph when the target is not a source asset', () => {
    for (const line of ['![A map](https://example.com/x.png)', '![A map](assets/../source.txt)']) {
      const block = firstBlock(line);
      expect(block.type, line).toBe('paragraph');
      expect(block, line).toMatchObject({ segments: [{ text: line }] });
    }
  });

  it('renders an image whose file name contains a space', () => {
    expect(firstBlock('![A map](assets/A Map.JPEG)')).toEqual({
      type: 'image',
      name: 'A Map.JPEG',
      alt: 'A map'
    });
  });

  it('does not treat an image line inside a code fence as an image', () => {
    const blocks = parseMarkdownBlocks('```\n![A map](assets/img-0001.png)\n```');

    expect(blocks).toEqual([{ type: 'code', text: '![A map](assets/img-0001.png)' }]);
  });

  it('keeps incompatible and shorter fence runs inside the code block', () => {
    const blocks = parseMarkdownBlocks('````md\n~~~\n```\n## code\n````\n## Real');
    expect(blocks).toEqual([
      { type: 'code', text: '~~~\n```\n## code' },
      { type: 'heading', level: 2, segments: [{ text: 'Real' }] }
    ]);
  });

  it('keeps a four-space-indented ATX-looking line as prose', () => {
    const block = firstBlock('    ## code example');
    expect(block.type).toBe('paragraph');
  });
});

// An offline download uses this to decide what to fetch, so it has to agree
// exactly with what the reader renders: a link left as text must not be
// downloaded, and a rendered one must not be missed.
describe('referencedAssetNames', () => {
  it('lists rendered illustrations once, in first-use order', () => {
    const names = referencedAssetNames(
      ['![a](assets/b.png)', '', 'prose', '', '![c](assets/a.jpg)', '', '![again](assets/b.png)'].join('\n')
    );

    expect(names).toEqual(['b.png', 'a.jpg']);
  });

  it('skips targets the reader leaves as text', () => {
    const names = referencedAssetNames(
      [
        '![ext](https://example.com/x.png)',
        '',
        '![up](assets/../source.txt)',
        '',
        '![inline](assets/ok.png) in a sentence',
        '',
        '![kept](assets/ok.png)'
      ].join('\n')
    );

    expect(names).toEqual(['ok.png']);
  });

  it('returns nothing for a text carrying no illustrations', () => {
    expect(referencedAssetNames('# Title\n\njust prose')).toEqual([]);
  });
});
