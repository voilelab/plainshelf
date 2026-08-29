export const MOBILE_READER_TAP_DISTANCE = 10;
export const MOBILE_READER_TAP_DURATION_MS = 350;
export const MOBILE_READER_SWIPE_DISTANCE = 60;
export const MOBILE_READER_SWIPE_AXIS_RATIO = 1.5;

export type ReaderGesture = 'tap' | 'previous' | 'next' | 'none';

export interface ReaderGestureInput {
  deltaX: number;
  deltaY: number;
  maxMovement: number;
  durationMs: number;
}

export interface ReaderTapPoint {
  clientX: number;
  clientY: number;
}

export interface ReaderBounds {
  left: number;
  top: number;
  width: number;
  height: number;
}

export function classifyReaderGesture(input: ReaderGestureInput): ReaderGesture {
  const absX = Math.abs(input.deltaX);
  const absY = Math.abs(input.deltaY);

  if (absX >= MOBILE_READER_SWIPE_DISTANCE && absX >= absY * MOBILE_READER_SWIPE_AXIS_RATIO) {
    return input.deltaX < 0 ? 'next' : 'previous';
  }

  if (
    input.maxMovement <= MOBILE_READER_TAP_DISTANCE &&
    input.durationMs <= MOBILE_READER_TAP_DURATION_MS
  ) {
    return 'tap';
  }

  return 'none';
}

// The chrome-toggle zone is a vertical band, not a centered square: the two far
// horizontal edges stay reserved for the page-turn taps, but the band spans the
// full height so a tap low on the screen — where a thumb naturally lands — still
// toggles the toolbars instead of falling into dead space.
export function isInReaderCenter(point: ReaderTapPoint, bounds: ReaderBounds): boolean {
  if (bounds.width <= 0 || bounds.height <= 0) {
    return false;
  }

  const minX = bounds.left + bounds.width * 0.25;
  const maxX = bounds.left + bounds.width * 0.75;

  return point.clientX >= minX && point.clientX <= maxX;
}

export function isReaderInteractiveTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) {
    return false;
  }

  return Boolean(
    target.closest(
      'a, button, input, select, textarea, summary, .reader-md-code, [contenteditable="true"], [role="button"], [role="link"]'
    )
  );
}
