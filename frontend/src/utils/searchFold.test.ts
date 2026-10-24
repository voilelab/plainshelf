import { describe, expect, it } from 'vitest';
import { foldForSearch } from './searchFold';

function sameFold(a: string, b: string): void {
  expect(foldForSearch(a)).toBe(foldForSearch(b));
}

describe('foldForSearch', () => {
  it('folds Traditional and Simplified Chinese to one form', () => {
    sameFold('書名', '书名');
    sameFold('臺灣', '台湾');
    sameFold('頭髮', '头发');
  });

  it('folds Japanese shinjitai and kyujitai to the same form as Chinese', () => {
    sameFold('桜', '櫻');
    sameFold('桜', '樱');
    sameFold('学校', '學校');
    sameFold('気', '氣');
  });

  it('is idempotent', () => {
    for (const text of ['薴', '苧', '學', '桜', 'Ｃａｆé', 'ｶﾞｸ']) {
      expect(foldForSearch(foldForSearch(text))).toBe(foldForSearch(text));
    }
  });

  it('folds fullwidth and halfwidth forms', () => {
    sameFold('ＡＢＣ１２３', 'abc123');
    sameFold('ｶﾞｸ', 'がく');
  });

  it('folds composed and decomposed sequences', () => {
    sameFold('café', 'café');
    sameFold('한', '한');
  });

  it('removes Latin diacritics', () => {
    sameFold('Brontë', 'bronte');
    sameFold('Dvořák', 'dvorak');
    sameFold('İstanbul', 'istanbul');
  });

  it('folds letters that have no decomposition', () => {
    sameFold('Straße', 'STRASSE');
    sameFold('Łódź', 'lodz');
    sameFold('Ørsted', 'orsted');
    sameFold('λόγος', 'ΛΌΓΟΣ');
  });

  it('keeps Japanese voiced marks', () => {
    expect(foldForSearch('が')).not.toBe(foldForSearch('か'));
  });

  it('folds katakana to hiragana', () => {
    sameFold('カタカナ', 'かたかな');
  });

  it('treats name separators and whitespace runs as one space', () => {
    sameFold('約翰·史密斯', '約翰・史密斯');
    sameFold('約翰‧史密斯', '约翰 史密斯');
    sameFold('a　  b', 'a b');
  });

  it('drops invisible characters', () => {
    sameFold('pl­ain​shelf', 'plainshelf');
    sameFold('葛\u{e0100}', '葛');
  });
});
