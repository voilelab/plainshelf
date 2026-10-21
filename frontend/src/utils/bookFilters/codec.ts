
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

export type FilterFieldOp = 'all';

export function serializeFilterFieldOp(op: FilterFieldOp): string {
  return op;
}

export function parseFilterFieldOp(raw: string | undefined): FilterFieldOp | undefined {
  return raw === 'all' ? 'all' : undefined;
}

export function filterFieldOpKey(field: string): string {
  return `${field}Op`;
}
