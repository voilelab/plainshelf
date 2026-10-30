
export type ScanIntervalMode = 'default' | 'interval' | 'always';
export type ScanIntervalUnit = 's' | 'm' | 'h';

interface ScanIntervalSelection {
  mode: ScanIntervalMode;
  amount: number;
  unit: ScanIntervalUnit;
}

interface ScanIntervalLoad extends ScanIntervalSelection {
  adjustedFrom: string;
}

const DEFAULT_AMOUNT = 1;
const DEFAULT_UNIT: ScanIntervalUnit = 'm';

const UNIT_MS: Record<string, number> = {
  ns: 1e-6,
  us: 1e-3,
  // Go time.ParseDuration accepts both µs and μs for microseconds.
  'µs': 1e-3,
  'μs': 1e-3,
  ms: 1,
  s: 1000,
  m: 60_000,
  h: 3_600_000
};

const SECONDS_PER_UNIT: Record<ScanIntervalUnit, number> = { s: 1, m: 60, h: 3600 };

// Go time.Duration is signed int64 nanoseconds and time.ParseDuration rejects overflow; ECMAScript Number cannot represent the exact upper bound.
const MAX_DURATION_MS = 9_223_372_036_854.775;

export function maxScanIntervalAmount(unit: ScanIntervalUnit): number {
  return Math.floor(MAX_DURATION_MS / (SECONDS_PER_UNIT[unit] * 1000));
}

const TERM_PATTERN = /^(\d*\.?\d*)(ns|us|µs|μs|ms|s|m|h)/;

export function parseGoDuration(text: string): number | null {
  let rest = text.trim();
  if (rest === '') {
    return null;
  }

  let sign = 1;
  if (rest.startsWith('+') || rest.startsWith('-')) {
    sign = rest.startsWith('-') ? -1 : 1;
    rest = rest.slice(1);
  }

  // Go time.ParseDuration accepts bare zero without a unit.
  if (rest === '0') {
    return 0;
  }

  let total = 0;
  while (rest !== '') {
    const term = TERM_PATTERN.exec(rest);
    if (!term) {
      return null;
    }
    const [matched, digits, unit] = term;
    if (digits === '' || digits === '.') {
      return null;
    }
    total += Number(digits) * UNIT_MS[unit];
    rest = rest.slice(matched.length);
  }

  if (!Number.isFinite(total) || total > MAX_DURATION_MS) {
    return null;
  }

  return sign * total;
}

export function scanIntervalFromSelection(selection: ScanIntervalSelection): string {
  if (selection.mode === 'default') {
    return '';
  }
  if (selection.mode === 'always') {
    return '0s';
  }

  const amount = Math.min(
    maxScanIntervalAmount(selection.unit),
    Math.max(1, Math.floor(selection.amount))
  );
  return `${amount}${selection.unit}`;
}

export function scanIntervalToSelection(stored: string): ScanIntervalLoad {
  const raw = stored.trim();
  if (raw === '') {
    return { mode: 'default', amount: DEFAULT_AMOUNT, unit: DEFAULT_UNIT, adjustedFrom: '' };
  }

  const ms = parseGoDuration(raw);
  if (ms === null) {
    return { mode: 'default', amount: DEFAULT_AMOUNT, unit: DEFAULT_UNIT, adjustedFrom: raw };
  }
  if (ms <= 0) {
    return { mode: 'always', amount: DEFAULT_AMOUNT, unit: DEFAULT_UNIT, adjustedFrom: ms < 0 ? raw : '' };
  }

  const seconds = Math.max(1, Math.round(ms / 1000));
  const exact = seconds * 1000 === ms;

  let unit: ScanIntervalUnit = 's';
  if (seconds % SECONDS_PER_UNIT.h === 0) {
    unit = 'h';
  } else if (seconds % SECONDS_PER_UNIT.m === 0) {
    unit = 'm';
  }

  return {
    mode: 'interval',
    amount: seconds / SECONDS_PER_UNIT[unit],
    unit,
    adjustedFrom: exact ? '' : raw
  };
}
