/**
 * URL value grammar shared by the declarative book filters: one opaque token per
 * field, so a new presence/equality condition needs no encoding of its own.
 *
 * - `none` — the field is absent/empty on the book.
 * - `has`  — the field is present on the book.
 * - `eq:<value>` — the field equals `<value>`, taken verbatim after the first
 *   `eq:`. The prefix is what disambiguates a literal value from the sentinels,
 *   so `eq:none` round-trips to the string "none", and colons or commas inside
 *   `<value>` need no escaping.
 *
 * Anything else parses to `undefined`, so a stale `?field=garbage` is ignored
 * rather than throwing — the contract `toBookSort` gives an unknown sort key.
 */

export type FilterFieldValue =
  | { readonly kind: 'none' }
  | { readonly kind: 'has' }
  | { readonly kind: 'eq'; readonly value: string };

const EQ_PREFIX = 'eq:';

export function serializeFilterField(value: FilterFieldValue): string {
  switch (value.kind) {
    case 'none':
      return 'none';
    case 'has':
      return 'has';
    case 'eq':
      return `${EQ_PREFIX}${value.value}`;
  }
}

/**
 * `undefined` when the token is not one of the three forms above.
 * `parse(serialize(v))` is deep-equal to `v` for every `v`.
 */
export function parseFilterField(raw: string | undefined): FilterFieldValue | undefined {
  if (raw === undefined) {
    return undefined;
  }
  if (raw === 'none') {
    return { kind: 'none' };
  }
  if (raw === 'has') {
    return { kind: 'has' };
  }
  if (raw.startsWith(EQ_PREFIX)) {
    return { kind: 'eq', value: raw.slice(EQ_PREFIX.length) };
  }
  return undefined;
}

/**
 * Multi-value combine operator carried in the sibling `<field>Op` key. Only
 * `all` (AND) is accepted today, but the key is parsed now because both
 * intuitions for repeated values exist in the wild — GitHub reads repetition as
 * AND, faceted search as OR — and pinning it in the URL means a later `any` can
 * be a new value instead of silently redefining an existing bookmark.
 */
export type FilterFieldOp = 'all';

export function serializeFilterFieldOp(op: FilterFieldOp): string {
  return op;
}

/** Decodes an `<field>Op` token, or `undefined` for an absent/unknown operator. */
export function parseFilterFieldOp(raw: string | undefined): FilterFieldOp | undefined {
  return raw === 'all' ? 'all' : undefined;
}

export function filterFieldOpKey(field: string): string {
  return `${field}Op`;
}
