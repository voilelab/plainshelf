import { describe, expect, it } from 'vitest';

import {
  assetNameFromSrc,
  parseMarkdownBlocks,
  parseMarkdownHeadingLine,
  referencedAssetNames,
  updateMarkdownFenceState,
  type MarkdownBlock
} from './parseMarkdownBlocks';
import { rewriteMarkdownAssetImages } from './markdownAssetImages';

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

  // The supported extension has to end the name and the name has to end the
  // path; otherwise "assets/x.png.txt" and "assets/x.png/passwd" both read as
  // an image and the reader opens whatever they really point at.
  it('anchors both the assets/ path and the extension at the end', () => {
    for (const src of [
      'assets/img-0001.png/nested.txt',
      'assets/img-0001.png/',
      'assets/img-0001.png.txt',
      'assets/img-0001.pngx'
    ]) {
      expect(assetNameFromSrc(src), src).toBeNull();
    }
  });

  // CommonMark allows padding around a destination and inside its angle
  // brackets; a file named in either spelling is the same file.
  it('ignores the whitespace CommonMark allows around a destination', () => {
    expect(assetNameFromSrc('  assets/img-0001.png  ')).toBe('img-0001.png');
    expect(assetNameFromSrc('< assets/img-0001.png >')).toBe('img-0001.png');
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

  // Only a line that is nothing but an image becomes an illustration, so an
  // image with prose on either side has to stay in its sentence.
  it('leaves an image with text on only one side as text', () => {
    for (const line of [
      'see ![A map](assets/img-0001.png)',
      '![A map](assets/img-0001.png) here'
    ]) {
      const block = firstBlock(line);
      expect(block.type, line).toBe('paragraph');
      expect(block, line).toMatchObject({ segments: [{ text: line }] });
    }
  });

  it('trims the alt text an author padded', () => {
    expect(firstBlock('![  A map  ](assets/img-0001.png)')).toEqual({
      type: 'image',
      name: 'img-0001.png',
      alt: 'A map'
    });
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

  it('includes an indented list continuation image that the reader renders', () => {
    expect(referencedAssetNames('- Floor plan\n  ![map](assets/map.png)')).toEqual(['map.png']);
  });

  it('skips an image-looking line in raw HTML that Markdown leaves literal', () => {
    expect(referencedAssetNames('<pre>\n![map](assets/map.png)\n</pre>')).toEqual([]);
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

// These two helpers are the shared line-level syntax: the chapter scanner, the
// reader's renderer, and the source converter all read a document through them,
// so a disagreement here shows up as three different bugs.
describe('parseMarkdownHeadingLine', () => {
  it('reads the level and the title of an ATX heading', () => {
    expect(parseMarkdownHeadingLine('## Part 1')).toEqual({ level: 2, title: 'Part 1' });
    expect(parseMarkdownHeadingLine('   ###### Deep')).toEqual({ level: 6, title: 'Deep' });
    expect(parseMarkdownHeadingLine('##\tTabbed')).toEqual({ level: 2, title: 'Tabbed' });
  });

  it('trims the padding around a title', () => {
    expect(parseMarkdownHeadingLine('##   Part 1   ')).toEqual({ level: 2, title: 'Part 1' });
  });

  it('refuses the lines that only look like headings', () => {
    for (const line of ['#NoSpace', '####### Seven', '    ## Indented code', '## ', 'Body #']) {
      expect(parseMarkdownHeadingLine(line), line).toBeNull();
    }
  });
});

describe('updateMarkdownFenceState', () => {
  const open = (line: string) => updateMarkdownFenceState(line, null);
  const inside = (line: string, marker: '`' | '~', length: number) =>
    updateMarkdownFenceState(line, { marker, length });

  it('opens on a fence line and reports the run it has to be closed with', () => {
    expect(open('```')).toEqual({ state: { marker: '`', length: 3 }, boundary: true });
    expect(open('~~~~')).toEqual({ state: { marker: '~', length: 4 }, boundary: true });
    expect(open('   ```js')).toEqual({ state: { marker: '`', length: 3 }, boundary: true });
    expect(open('Body')).toEqual({ state: null, boundary: false });
  });

  // A backtick inside the info string is not a fence in CommonMark. Reading it
  // as one would swallow the rest of the document as code.
  it('does not open a backtick fence whose info string contains a backtick', () => {
    expect(open('```a`b')).toEqual({ state: null, boundary: false });
    expect(open('~~~a`b')).toEqual({ state: { marker: '~', length: 3 }, boundary: true });
  });

  // A run appearing after text on the line is inline code. Opening a fence
  // there would read the rest of the document as one code block.
  it('does not open a fence on a run that follows text', () => {
    expect(open('text ```js')).toEqual({ state: null, boundary: false });
    expect(open('    ```js')).toEqual({ state: null, boundary: false });
  });

  it('closes only on the same marker with at least the opening run length', () => {
    expect(inside('```', '`', 3)).toEqual({ state: null, boundary: true });
    expect(inside('~~~', '~', 3)).toEqual({ state: null, boundary: true });
    expect(inside('````', '`', 3)).toEqual({ state: null, boundary: true });
    expect(inside('```', '`', 4)).toEqual({ state: { marker: '`', length: 4 }, boundary: false });
    expect(inside('~~~', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
  });

  // The closing line may carry trailing whitespace and nothing else, and it
  // has to start the line: a fence run appearing after code text is code.
  it('reads a closing line strictly', () => {
    expect(inside('```  ', '`', 3)).toEqual({ state: null, boundary: true });
    expect(inside('   ```\t', '`', 3)).toEqual({ state: null, boundary: true });
    expect(inside('``` js', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
    expect(inside('code```', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
    expect(inside('    ```', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
  });
});

describe('rewriteMarkdownAssetImages', () => {
  // Offline collection calls this without a rewrite to read the image list off
  // an unchanged document; a default that edited the text would corrupt it.
  it('returns the text unchanged when no rewrite is given', () => {
    const text = '![a](assets/b.png)\n\nprose\n\n![c](assets/d.jpg)';
    expect(rewriteMarkdownAssetImages(text)).toEqual({
      text,
      images: [{ name: 'b.png', alt: 'a' }, { name: 'd.jpg', alt: 'c' }]
    });
  });

  it('replaces only the image lines and keeps their order', () => {
    const result = rewriteMarkdownAssetImages(
      'before\n\n![a](assets/b.png)\n\n![ext](https://example.com/x.png)',
      (image, index) => `<slot ${index} ${image.name}>`
    );
    expect(result.text).toBe('before\n\n<slot 0 b.png>\n\n![ext](https://example.com/x.png)');
    expect(result.images).toEqual([{ name: 'b.png', alt: 'a' }]);
  });
});
