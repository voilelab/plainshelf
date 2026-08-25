/**
 * URL value grammar shared by the declarative book filters.
 *
 * A filter field is carried in the query string as a single opaque token so a
 * new presence/equality condition can be added without inventing (and later
 * having to migrate) its own encoding. Three forms are understood:
 *
 * - `none` — the field is absent/empty on the book.
 * - `has`  — the field is present on the book.
 * - `eq:<value>` — the field equals `<value>`, taken verbatim after the first
 *   `eq:`. The prefix is what disambiguates a literal value from the sentinels,
 *   so `eq:none` round-trips back to the string "none" rather than the "absent"
 *   sentinel, and colons or commas inside `<value>` need no escaping.
 *
 * Anything else parses to `undefined` so a hand-edited or stale `?field=garbage`
 * is ignored rather than throwing — the same forgiving contract `toBookSort`
 * gives an unknown sort key.
 */

export type FilterFieldValue =
  | { readonly kind: 'none' }
  | { readonly kind: 'has' }
  | { readonly kind: 'eq'; readonly value: string };

const EQ_PREFIX = 'eq:';

/** Encodes a field value into its single query-string token. */
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
 * Decodes a query-string token back into a field value, or `undefined` when the
 * token is not one of the three understood forms. `parse(serialize(v))` returns
 * a value deep-equal to `v` for every `v`.
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
 * Multi-value combine operator carried in the sibling `<field>Op` key.
 *
 * Only `all` (every listed value must match — AND) is accepted today, but the
 * key is parsed now so the default multi-value meaning stays explicit. Both
 * intuitions for repeated values exist in the wild (GitHub treats repetition as
 * AND, faceted-search UIs as OR); pinning the operator in the URL means a later
 * `any` (OR) can be added as a new value instead of silently redefining what an
 * existing bookmark meant.
 */
export type FilterFieldOp = 'all';

/** Encodes the combine operator into its `<field>Op` token. */
export function serializeFilterFieldOp(op: FilterFieldOp): string {
  return op;
}

/** Decodes an `<field>Op` token, or `undefined` for an absent/unknown operator. */
export function parseFilterFieldOp(raw: string | undefined): FilterFieldOp | undefined {
  return raw === 'all' ? 'all' : undefined;
}

/** The sibling query key that carries a field's combine operator. */
export function filterFieldOpKey(field: string): string {
  return `${field}Op`;
}
