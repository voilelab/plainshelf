import { describe, expect, it } from 'vitest';

import {
  assetImageFromMarkdownLine,
  assetNameFromSrc,
  parseMarkdownHeadingLine,
  referencedAssetNames,
  updateMarkdownFenceState
} from './markdownLineSyntax';
import { rewriteMarkdownAssetImages } from './markdownAssetImages';

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

  it('ignores the whitespace CommonMark allows around a destination', () => {
    expect(assetNameFromSrc('  assets/img-0001.png  ')).toBe('img-0001.png');
    expect(assetNameFromSrc('< assets/img-0001.png >')).toBe('img-0001.png');
  });

  it('accepts every spelling of a name containing a space', () => {
    for (const src of ['assets/A Map.JPEG', 'assets/A%20Map.JPEG', '<assets/A Map.JPEG>']) {
      expect(assetNameFromSrc(src), src).toBe('A Map.JPEG');
    }
  });

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

describe('assetImageFromMarkdownLine', () => {
  it('reads the name and alt of a line that is only an image', () => {
    expect(assetImageFromMarkdownLine('![A map](assets/img-0001.png)')).toEqual({
      name: 'img-0001.png',
      alt: 'A map'
    });
  });

  it('keeps an empty alt', () => {
    expect(assetImageFromMarkdownLine('![](assets/img-0001.png)')).toEqual({
      name: 'img-0001.png',
      alt: ''
    });
  });

  it('trims the alt text an author padded', () => {
    expect(assetImageFromMarkdownLine('![  A map  ](assets/img-0001.png)')).toEqual({
      name: 'img-0001.png',
      alt: 'A map'
    });
  });

  it('reads an image whose file name contains a space', () => {
    expect(assetImageFromMarkdownLine('![A map](assets/A Map.JPEG)')).toEqual({
      name: 'A Map.JPEG',
      alt: 'A map'
    });
  });

  it('refuses a target that is not a source asset', () => {
    for (const line of ['![A map](https://example.com/x.png)', '![A map](assets/../source.txt)']) {
      expect(assetImageFromMarkdownLine(line), line).toBeNull();
    }
  });

  it('refuses an image that shares its line with prose', () => {
    for (const line of [
      'see ![A map](assets/img-0001.png) here',
      'see ![A map](assets/img-0001.png)',
      '![A map](assets/img-0001.png) here'
    ]) {
      expect(assetImageFromMarkdownLine(line), line).toBeNull();
    }
  });
});

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

  it('does not open a backtick fence whose info string contains a backtick', () => {
    expect(open('```a`b')).toEqual({ state: null, boundary: false });
    expect(open('~~~a`b')).toEqual({ state: { marker: '~', length: 3 }, boundary: true });
  });

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

  it('reads a closing line strictly', () => {
    expect(inside('```  ', '`', 3)).toEqual({ state: null, boundary: true });
    expect(inside('   ```\t', '`', 3)).toEqual({ state: null, boundary: true });
    expect(inside('``` js', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
    expect(inside('code```', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
    expect(inside('    ```', '`', 3)).toEqual({ state: { marker: '`', length: 3 }, boundary: false });
  });
});

describe('rewriteMarkdownAssetImages', () => {
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
