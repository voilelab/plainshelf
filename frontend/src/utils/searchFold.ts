import jpShinjitaiCharacters from 'opencc-js/dict/JPShinjitaiCharacters';
import tsCharacters from 'opencc-js/dict/TSCharacters';

type CharMap = Map<string, string>;

const MAX_HAN_FOLD_STEPS = 4;
const KATAKANA_TO_HIRAGANA_OFFSET = 0x60;

const IGNORABLE = /\p{Default_Ignorable_Code_Point}/gu;
const LATIN_COMBINING_MARKS = /[̀-ͯ]/g;
const NAME_SEPARATORS = /[·・‧•∙⋅]/g;
const WHITESPACE_RUN = /\s+/g;
const KATAKANA = /[ァ-ヶ]/g;

const UNDECOMPOSABLE_LETTERS: Readonly<Record<string, string>> = {
  ß: 'ss',
  ς: 'σ',
  æ: 'ae',
  œ: 'oe',
  ø: 'o',
  ł: 'l',
  đ: 'd'
};
const UNDECOMPOSABLE_LETTER = new RegExp(`[${Object.keys(UNDECOMPOSABLE_LETTERS).join('')}]`, 'g');

function parseOpenCCCharacters(dict: string): CharMap {
  return new Map(dict.split('|').map((entry) => {
    const [from, to] = entry.split(' ');
    return [from, to] as [string, string];
  }));
}

function buildHanFoldTable(traditionalToSimplified: CharMap, shinjitaiToKyujitai: CharMap): CharMap {
  const simplifiedForms = new Set(traditionalToSimplified.values());
  const foldOnce = (ch: string): string => {
    const simplified = traditionalToSimplified.get(ch);
    if (simplified !== undefined) {
      return simplified;
    }
    if (simplifiedForms.has(ch)) {
      return ch;
    }
    const kyujitai = shinjitaiToKyujitai.get(ch);
    return kyujitai === undefined ? ch : traditionalToSimplified.get(kyujitai) ?? kyujitai;
  };

  const table: CharMap = new Map();
  for (const ch of new Set([...traditionalToSimplified.keys(), ...shinjitaiToKyujitai.keys()])) {
    let folded = ch;
    for (let step = 0; step < MAX_HAN_FOLD_STEPS; step += 1) {
      const next = foldOnce(folded);
      if (next === folded) {
        break;
      }
      folded = next;
    }
    if (folded !== ch) {
      table.set(ch, folded);
    }
  }
  return table;
}

const hanFoldTable = buildHanFoldTable(
  parseOpenCCCharacters(tsCharacters),
  parseOpenCCCharacters(jpShinjitaiCharacters)
);

function foldHan(text: string): string {
  let out = '';
  for (const ch of text) {
    out += hanFoldTable.get(ch) ?? ch;
  }
  return out;
}

export function foldForSearch(text: string): string {
  const compatible = text
    .normalize('NFKC')
    .replace(IGNORABLE, '')
    .toLowerCase()
    .replace(UNDECOMPOSABLE_LETTER, (ch) => UNDECOMPOSABLE_LETTERS[ch]);
  const withoutDiacritics = compatible
    .normalize('NFD')
    .replace(LATIN_COMBINING_MARKS, '')
    .normalize('NFC');
  const hiragana = withoutDiacritics.replace(KATAKANA, (ch) =>
    String.fromCharCode(ch.charCodeAt(0) - KATAKANA_TO_HIRAGANA_OFFSET)
  );
  return foldHan(hiragana)
    .replace(NAME_SEPARATORS, ' ')
    .replace(WHITESPACE_RUN, ' ')
    .trim();
}
