import en from './locales/en';

const CATALOG_SECTIONS: readonly string[] = Object.keys(en);

export const MISSING_KEY_PATTERN = new RegExp(
  `\\b(${CATALOG_SECTIONS.join('|')})(\\.[a-zA-Z][a-zA-Z0-9]*)+\\b`
);
