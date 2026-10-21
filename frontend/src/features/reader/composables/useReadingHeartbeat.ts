import { watch } from 'vue';

import { addReadingSeconds } from '@/storage/readingStats';

const TICK_INTERVAL_MS = 45_000;
const IDLE_TIMEOUT_MS = 5 * 60 * 1000;
const TIMER_COALESCING_TOLERANCE_MS = 23_000;
const MAX_REPORTABLE_SECONDS = (TICK_INTERVAL_MS + TIMER_COALESCING_TOLERANCE_MS) / 1000;

// CSSOM View scrolling events do not bubble from elements; document listeners need capture to observe descendant scrolling (https://drafts.csswg.org/cssom-view/#scrolling-events).
const INTERACTION_EVENT_NAMES: Array<keyof DocumentEventMap> = ['scroll', 'keydown', 'pointerdown'];

function toIsoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function clampSeconds(seconds: number): number {
  if (!Number.isFinite(seconds)) {
    return 0;
  }
  return Math.max(0, Math.min(MAX_REPORTABLE_SECONDS, seconds));
}

export function useReadingHeartbeat(bookID: () => string) {
  let intervalId: ReturnType<typeof setInterval> | null = null;
  let idleTimer: ReturnType<typeof setTimeout> | null = null;
  let activeStretchStartedAt = 0;
  let hidden = false;
  let idle = false;
  let accumulatingBookID = '';

  function report(seconds: number): void {
    const rounded = clampSeconds(seconds);
    if (rounded <= 0 || accumulatingBookID === '') {
      return;
    }

    const date = toIsoDate(new Date());
    addReadingSeconds(accumulatingBookID, rounded, date).catch((err) => {
      console.warn('Failed to record reading activity', err);
    });
  }

  watch(bookID, (newBookID) => {
    if (intervalId !== null && !hidden && !idle) {
      const now = Date.now();
      report((now - activeStretchStartedAt) / 1000);
    }
    activeStretchStartedAt = Date.now();
    accumulatingBookID = newBookID;
  });

  function resetIdleTimer(): void {
    if (idleTimer !== null) {
      clearTimeout(idleTimer);
    }
    idleTimer = setTimeout(() => {
      idle = true;
    }, IDLE_TIMEOUT_MS);
  }

  function onInteraction(): void {
    if (idle) {
      idle = false;
      activeStretchStartedAt = Date.now();
    }
    resetIdleTimer();
  }

  function onVisibilityChange(): void {
    if (document.hidden) {
      hidden = true;
      return;
    }
    hidden = false;
    activeStretchStartedAt = Date.now();
  }

  function tick(): void {
    if (hidden || idle) {
      return;
    }
    const now = Date.now();
    const elapsedSeconds = (now - activeStretchStartedAt) / 1000;
    activeStretchStartedAt = now;
    report(elapsedSeconds);
  }

  function start(): void {
    accumulatingBookID = bookID();
    activeStretchStartedAt = Date.now();
    hidden = document.hidden;
    idle = false;

    intervalId = setInterval(tick, TICK_INTERVAL_MS);
    document.addEventListener('visibilitychange', onVisibilityChange);
    for (const eventName of INTERACTION_EVENT_NAMES) {
      document.addEventListener(eventName, onInteraction, { capture: true });
    }
    resetIdleTimer();
  }

  function stop(): void {
    if (intervalId !== null) {
      clearInterval(intervalId);
      intervalId = null;
    }
    if (idleTimer !== null) {
      clearTimeout(idleTimer);
      idleTimer = null;
    }
    document.removeEventListener('visibilitychange', onVisibilityChange);
    for (const eventName of INTERACTION_EVENT_NAMES) {
      document.removeEventListener(eventName, onInteraction, { capture: true });
    }

    if (!hidden && !idle) {
      const elapsedSeconds = (Date.now() - activeStretchStartedAt) / 1000;
      report(elapsedSeconds);
    }
  }

  return { start, stop };
}
