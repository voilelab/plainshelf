/**
 * The shelf scan interval as the settings UI edits it.
 *
 * On disk the value is a Go duration string (`shelf.ShelfConf.ScanInterval`,
 * parsed by `time.ParseDuration`), where an empty string means the built-in
 * default and `0s` means "no throttling, walk the whole shelf on every
 * refresh". Neither is guessable from a text box, so the panel edits a mode
 * plus an amount and a unit and converts here.
 */

export type ScanIntervalMode = 'default' | 'interval' | 'always';
export type ScanIntervalUnit = 's' | 'm' | 'h';

export interface ScanIntervalSelection {
  mode: ScanIntervalMode;
  /** Whole units; only meaningful for the `interval` mode. */
  amount: number;
  unit: ScanIntervalUnit;
}

export interface ScanIntervalLoad extends ScanIntervalSelection {
  /**
   * The stored string when the controls cannot represent it exactly (sub-second
   * precision, a negative duration, or something `time.ParseDuration` would
   * reject), so the caller can say the value was replaced. Empty when the
   * selection round-trips to the same duration.
   */
  adjustedFrom: string;
}

/** Matches `time.ParseDuration`'s default of one minute in `NewShelf`. */
const DEFAULT_AMOUNT = 1;
const DEFAULT_UNIT: ScanIntervalUnit = 'm';

const UNIT_MS: Record<string, number> = {
  ns: 1e-6,
  us: 1e-3,
  // Both the micro sign and the Greek letter, as Go accepts.
  'µs': 1e-3,
  'μs': 1e-3,
  ms: 1,
  s: 1000,
  m: 60_000,
  h: 3_600_000
};

const SECONDS_PER_UNIT: Record<ScanIntervalUnit, number> = { s: 1, m: 60, h: 3600 };

// Longest-first so `ms` is not read as `m` followed by a stray `s`.
const TERM_PATTERN = /^(\d*\.?\d*)(ns|us|µs|μs|ms|s|m|h)/;

/**
 * Parses a Go duration string into milliseconds, or null when
 * `time.ParseDuration` would reject it. Kept deliberately close to Go's own
 * grammar so the UI accepts exactly the values the shelf already holds.
 */
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

  // Go's one special case: a bare zero needs no unit.
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

  return sign * total;
}

/** Renders a selection as the Go duration string the backend stores. */
export function scanIntervalFromSelection(selection: ScanIntervalSelection): string {
  if (selection.mode === 'default') {
    return '';
  }
  if (selection.mode === 'always') {
    return '0s';
  }

  const amount = Math.max(1, Math.floor(selection.amount));
  return `${amount}${selection.unit}`;
}

/**
 * Reads a stored duration back into the controls, normalizing as it goes: `60s`
 * loads as one minute and `1h30m` as ninety minutes, because the controls carry
 * a single unit. A value they cannot hold is replaced by the nearest one they
 * can and reported in `adjustedFrom`.
 */
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
    // A negative interval throttles nothing, same as 0s, but it is not the
    // string we would write back.
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
