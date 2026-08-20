import { describe, expect, it } from 'vitest';
import { parseReaderBlocks } from './parseReaderBlocks';

describe('parseReaderBlocks', () => {
  it('returns nothing for a section holding only whitespace', () => {
    for (const text of ['', '   ', '\n\n \n\t\n']) {
      expect(parseReaderBlocks(text), JSON.stringify(text)).toEqual([]);
    }
  });

  it('splits paragraphs on blank lines and keeps single line breaks inside one block', () => {
    expect(parseReaderBlocks('First line\nsame paragraph\n\nSecond paragraph')).toEqual([
      { type: 'paragraph', text: 'First line\nsame paragraph' },
      { type: 'paragraph', text: 'Second paragraph' }
    ]);
  });

  // Hand-edited Markdown routinely leaves several blank lines and trailing
  // spaces between paragraphs; neither may produce an empty rendered block.
  it('collapses runs of blank lines and trims each block', () => {
    expect(parseReaderBlocks('\n\n  One  \n\n\n\n  Two  \n\n')).toEqual([
      { type: 'paragraph', text: 'One' },
      { type: 'paragraph', text: 'Two' }
    ]);
  });

  it('turns a block whose every line is quoted into a quote and strips the markers', () => {
    expect(parseReaderBlocks('> Quoted\n>Tight\n>  Padded')).toEqual([
      { type: 'quote', text: 'Quoted\nTight\n Padded' }
    ]);
  });

  // The marker has to hold for the whole block: a partially quoted block is
  // prose that happens to contain a ">", and losing its markers would rewrite
  // the author's text.
  it('keeps a partially quoted block as a paragraph with its markers intact', () => {
    expect(parseReaderBlocks('> Quoted\nNot quoted')).toEqual([
      { type: 'paragraph', text: '> Quoted\nNot quoted' }
    ]);
  });

  it('quotes a block whose quoted lines are separated by a blank line inside it', () => {
    expect(parseReaderBlocks('> One\n\t\n> Two')).toEqual([
      { type: 'quote', text: 'One\nTwo' }
    ]);
  });

  it('classifies each block independently', () => {
    expect(parseReaderBlocks('Intro\n\n> Quoted\n\nOutro')).toEqual([
      { type: 'paragraph', text: 'Intro' },
      { type: 'quote', text: 'Quoted' },
      { type: 'paragraph', text: 'Outro' }
    ]);
  });

  // Only the marker and one optional space belong to the syntax; deeper
  // indentation is the quote's own content.
  it('removes exactly one leading space after the marker and keeps nesting visible', () => {
    expect(parseReaderBlocks('>> Nested\n> > Spaced')).toEqual([
      { type: 'quote', text: '> Nested\n> Spaced' }
    ]);
  });
});
