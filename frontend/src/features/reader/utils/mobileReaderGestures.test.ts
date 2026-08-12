import { describe, expect, it } from 'vitest';
import { classifyReaderGesture, isInReaderCenter } from './mobileReaderGestures';

describe('mobile reader gestures', () => {
  it('maps horizontal swipes to chapter navigation', () => {
    expect(classifyReaderGesture({ deltaX: -80, deltaY: 12, maxMovement: 81, durationMs: 240 })).toBe('next');
    expect(classifyReaderGesture({ deltaX: 80, deltaY: -12, maxMovement: 81, durationMs: 240 })).toBe('previous');
  });

  it('rejects short or mostly vertical movements', () => {
    expect(classifyReaderGesture({ deltaX: -59, deltaY: 0, maxMovement: 59, durationMs: 200 })).toBe('none');
    expect(classifyReaderGesture({ deltaX: -70, deltaY: 60, maxMovement: 92, durationMs: 200 })).toBe('none');
    expect(classifyReaderGesture({ deltaX: 8, deltaY: 70, maxMovement: 70, durationMs: 200 })).toBe('none');
  });

  it('recognizes only short, stationary taps', () => {
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 4, durationMs: 200 })).toBe('tap');
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 11, durationMs: 200 })).toBe('none');
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 4, durationMs: 351 })).toBe('none');
  });

  it('limits toolbar toggling to the central half of the reader', () => {
    const bounds = { left: 20, top: 40, width: 400, height: 800 };
    expect(isInReaderCenter({ clientX: 120, clientY: 240 }, bounds)).toBe(true);
    expect(isInReaderCenter({ clientX: 320, clientY: 640 }, bounds)).toBe(true);
    expect(isInReaderCenter({ clientX: 119, clientY: 440 }, bounds)).toBe(false);
    expect(isInReaderCenter({ clientX: 220, clientY: 641 }, bounds)).toBe(false);
  });
});
