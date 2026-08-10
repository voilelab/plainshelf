import { describe, expect, it } from 'vitest';

import { assetNameFromSrc, parseMarkdownBlocks, type MarkdownBlock } from './parseMarkdownBlocks';

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

  it('does not treat an image line inside a code fence as an image', () => {
    const blocks = parseMarkdownBlocks('```\n![A map](assets/img-0001.png)\n```');

    expect(blocks).toEqual([{ type: 'code', text: '![A map](assets/img-0001.png)' }]);
  });
});
