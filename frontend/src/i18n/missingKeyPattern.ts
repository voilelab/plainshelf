import en from './locales/en';

/**
 * The catalog's own top-level sections. Derived rather than listed: the guard
 * below is only as wide as this array, and a hand-kept copy silently stops
 * covering a section the day one is added.
 */
const CATALOG_SECTIONS: readonly string[] = Object.keys(en);

/**
 * Matches the dotted path a missing key renders as. `t()` returns the key
 * itself when a lookup misses (see `./index.ts`) — it does not throw and does
 * not warn — so a typo'd or not-yet-added key reaches the screen as a literal
 * `pagination.firstPag`. `locales.test.ts` cannot see this: the catalogs are
 * consistent with each other, it is the *reference* that is wrong.
 *
 * Anchoring on the section names is what lets the pattern accept a two-segment
 * key without also matching ordinary content — this is a book library, so
 * `notes.md` as a title is a filename, not a bug.
 *
 * Two segments is the floor, not three: plenty of real keys are
 * `common.confirm` shaped, and an earlier version of this pattern required two
 * dots and so missed half the keys it was written to guard.
 */
export const MISSING_KEY_PATTERN = new RegExp(
  `\\b(${CATALOG_SECTIONS.join('|')})(\\.[a-zA-Z][a-zA-Z0-9]*)+\\b`
);
