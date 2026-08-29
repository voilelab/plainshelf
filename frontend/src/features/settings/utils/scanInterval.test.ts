import { describe, expect, it } from 'vitest';
import {
  parseGoDuration,
  scanIntervalFromSelection,
  scanIntervalToSelection
} from './scanInterval';

describe('parseGoDuration', () => {
  it('accepts the forms time.ParseDuration accepts', () => {
    expect(parseGoDuration('10m')).toBe(600_000);
    expect(parseGoDuration('1h30m')).toBe(5_400_000);
    expect(parseGoDuration('0s')).toBe(0);
    expect(parseGoDuration('0')).toBe(0);
    expect(parseGoDuration('1.5h')).toBe(5_400_000);
    expect(parseGoDuration('500ms')).toBe(500);
    expect(parseGoDuration('-10m')).toBe(-600_000);
    expect(parseGoDuration('100µs')).toBe(0.1);
  });

  it('rejects what the shelf would reject, including the bare number users type', () => {
    expect(parseGoDuration('10')).toBeNull();
    expect(parseGoDuration('10 min')).toBeNull();
    expect(parseGoDuration('5分鐘')).toBeNull();
    expect(parseGoDuration('')).toBeNull();
    expect(parseGoDuration('m')).toBeNull();
    expect(parseGoDuration('1e3s')).toBeNull();
  });
});

describe('scanIntervalToSelection', () => {
  it('loads a blank value as the shelf default', () => {
    expect(scanIntervalToSelection('')).toEqual({
      mode: 'default',
      amount: 1,
      unit: 'm',
      adjustedFrom: ''
    });
  });

  it('loads 0s as the "scan on every refresh" mode rather than a zero interval', () => {
    expect(scanIntervalToSelection('0s')).toEqual({
      mode: 'always',
      amount: 1,
      unit: 'm',
      adjustedFrom: ''
    });
  });

  it('normalizes to the largest unit that divides the duration', () => {
    expect(scanIntervalToSelection('10m')).toMatchObject({ mode: 'interval', amount: 10, unit: 'm' });
    expect(scanIntervalToSelection('60s')).toMatchObject({ mode: 'interval', amount: 1, unit: 'm' });
    expect(scanIntervalToSelection('90s')).toMatchObject({ mode: 'interval', amount: 90, unit: 's' });
    expect(scanIntervalToSelection('7200s')).toMatchObject({ mode: 'interval', amount: 2, unit: 'h' });
  });

  it('carries a compound duration as a single unit', () => {
    // 1h30m has no whole-hour form, so it comes back as ninety minutes - the
    // same duration, expressible by the controls.
    expect(scanIntervalToSelection('1h30m')).toEqual({
      mode: 'interval',
      amount: 90,
      unit: 'm',
      adjustedFrom: ''
    });
  });

  it('reports the values it cannot represent exactly', () => {
    expect(scanIntervalToSelection('1500ms')).toEqual({
      mode: 'interval',
      amount: 2,
      unit: 's',
      adjustedFrom: '1500ms'
    });
    // Under half a second still has to become a usable interval.
    expect(scanIntervalToSelection('200ms')).toMatchObject({
      mode: 'interval',
      amount: 1,
      unit: 's',
      adjustedFrom: '200ms'
    });
    // A negative interval throttles nothing, like 0s, but is not what we store.
    expect(scanIntervalToSelection('-5m')).toMatchObject({ mode: 'always', adjustedFrom: '-5m' });
    // Nothing time.ParseDuration accepts; fall back to the default and say so.
    expect(scanIntervalToSelection('10')).toMatchObject({ mode: 'default', adjustedFrom: '10' });
  });
});

describe('scanIntervalFromSelection', () => {
  it('writes the string the shelf config stores', () => {
    expect(scanIntervalFromSelection({ mode: 'default', amount: 1, unit: 'm' })).toBe('');
    expect(scanIntervalFromSelection({ mode: 'always', amount: 1, unit: 'm' })).toBe('0s');
    expect(scanIntervalFromSelection({ mode: 'interval', amount: 10, unit: 'm' })).toBe('10m');
    expect(scanIntervalFromSelection({ mode: 'interval', amount: 2, unit: 'h' })).toBe('2h');
  });

  it('clamps an amount the number box could hold but the shelf should not', () => {
    expect(scanIntervalFromSelection({ mode: 'interval', amount: 0, unit: 'm' })).toBe('1m');
    expect(scanIntervalFromSelection({ mode: 'interval', amount: -3, unit: 's' })).toBe('1s');
    expect(scanIntervalFromSelection({ mode: 'interval', amount: 1.7, unit: 's' })).toBe('1s');
  });

  it('round-trips every duration the controls can hold', () => {
    for (const stored of ['', '0s', '10m', '1h30m', '90s', '2h']) {
      const selection = scanIntervalToSelection(stored);
      const written = scanIntervalFromSelection(selection);
      expect(scanIntervalToSelection(written)).toEqual(selection);
    }
  });
});
