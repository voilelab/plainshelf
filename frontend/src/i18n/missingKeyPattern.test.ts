import { describe, expect, it } from 'vitest';
import en from './locales/en';
import { MISSING_KEY_PATTERN } from './missingKeyPattern';


describe('MISSING_KEY_PATTERN', () => {
  it.each([
    'pagination.firstPage',
    'common.confirm',
    'layout.foldersNavLabel',
    'settings.shelves.idColumn',
    'notFound.title'
  ])('matches %s, which is what a missed lookup renders', (key) => {
    expect(key).toMatch(MISSING_KEY_PATTERN);
  });

  it.each(['hello.txt', 'notes.md', 'book.epub', 'Settings', 'No books yet.'])(
    'leaves %s alone, because a library renders filenames as titles',
    (text) => {
      expect(text).not.toMatch(MISSING_KEY_PATTERN);
    }
  );

  it('matches a key inside a sentence, the way one reaches the screen', () => {
    expect('Delete trash.confirmTitle now?').toMatch(MISSING_KEY_PATTERN);
  });

  it('covers every section the catalog actually has', () => {
    const sections = Object.keys(en);

    expect(sections.length).toBeGreaterThan(10);
    for (const section of sections) {
      expect(`${section}.someKey`).toMatch(MISSING_KEY_PATTERN);
    }
  });
});
