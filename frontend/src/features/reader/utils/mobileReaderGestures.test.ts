import { afterEach, describe, expect, it } from 'vitest';
import { classifyReaderGesture, isInReaderCenter, isReaderInteractiveTarget } from './mobileReaderGestures';

const originalElement = globalThis.Element;

class TestElement {
  constructor(private readonly interactiveAncestor: string | null) {}

  closest(selector: string): TestElement | null {
    return this.interactiveAncestor && selector.includes(this.interactiveAncestor) ? this : null;
  }
}

afterEach(() => {
  Object.defineProperty(globalThis, 'Element', { configurable: true, value: originalElement });
});

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

  it('accepts a swipe that lands exactly on the distance and axis thresholds', () => {
    expect(classifyReaderGesture({ deltaX: -60, deltaY: 0, maxMovement: 60, durationMs: 200 })).toBe('next');
    expect(classifyReaderGesture({ deltaX: 60, deltaY: 40, maxMovement: 72, durationMs: 200 })).toBe('previous');
    expect(classifyReaderGesture({ deltaX: 60, deltaY: 41, maxMovement: 73, durationMs: 200 })).toBe('none');
  });

  it('recognizes only short, stationary taps', () => {
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 4, durationMs: 200 })).toBe('tap');
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 11, durationMs: 200 })).toBe('none');
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 4, durationMs: 351 })).toBe('none');
  });

  it('accepts a tap that lands exactly on the movement and duration limits', () => {
    expect(classifyReaderGesture({ deltaX: 2, deltaY: 3, maxMovement: 10, durationMs: 350 })).toBe('tap');
  });

  it('toggles chrome anywhere in the central vertical band', () => {
    const bounds = { left: 20, top: 40, width: 400, height: 800 };
    expect(isInReaderCenter({ clientX: 120, clientY: 240 }, bounds)).toBe(true);
    expect(isInReaderCenter({ clientX: 320, clientY: 640 }, bounds)).toBe(true);
    expect(isInReaderCenter({ clientX: 220, clientY: 41 }, bounds)).toBe(true);
    expect(isInReaderCenter({ clientX: 220, clientY: 839 }, bounds)).toBe(true);
  });

  it('toggles chrome for a tap near the bottom of the reader', () => {
    const bounds = { left: 0, top: 0, width: 400, height: 900 };
    expect(isInReaderCenter({ clientX: 200, clientY: 860 }, bounds)).toBe(true);
  });

  it('excludes the left and right edges of the reader at every height', () => {
    const bounds = { left: 20, top: 40, width: 400, height: 800 };
    expect(isInReaderCenter({ clientX: 119, clientY: 440 }, bounds)).toBe(false);
    expect(isInReaderCenter({ clientX: 321, clientY: 440 }, bounds)).toBe(false);
    expect(isInReaderCenter({ clientX: 119, clientY: 41 }, bounds)).toBe(false);
    expect(isInReaderCenter({ clientX: 321, clientY: 839 }, bounds)).toBe(false);
  });

  it('reports no center for a reader that has no area yet', () => {
    expect(isInReaderCenter({ clientX: 0, clientY: 400 }, { left: 0, top: 0, width: 0, height: 800 }))
      .toBe(false);
    expect(isInReaderCenter({ clientX: 200, clientY: 0 }, { left: 0, top: 0, width: 400, height: 0 }))
      .toBe(false);
    expect(isInReaderCenter({ clientX: 0, clientY: 0 }, { left: 0, top: 0, width: 0, height: 0 }))
      .toBe(false);
  });

  it('treats code blocks and their descendants as interactive gesture targets', () => {
    Object.defineProperty(globalThis, 'Element', { configurable: true, value: TestElement });

    const codeBlock = new TestElement('.reader-md-code');
    const codeChild = new TestElement('.reader-md-code');
    expect(isReaderInteractiveTarget(codeBlock as unknown as EventTarget)).toBe(true);
    expect(isReaderInteractiveTarget(codeChild as unknown as EventTarget)).toBe(true);
    expect(isReaderInteractiveTarget(new TestElement(null) as unknown as EventTarget)).toBe(false);
  });

  it('treats a target that is not an element as non-interactive', () => {
    Object.defineProperty(globalThis, 'Element', { configurable: true, value: TestElement });

    expect(isReaderInteractiveTarget(null)).toBe(false);
    expect(isReaderInteractiveTarget({ closest: () => ({}) } as unknown as EventTarget)).toBe(false);
  });
});
