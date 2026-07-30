import { watch } from 'vue';

import { getBookshelfProvider } from '@/providers';

const TICK_INTERVAL_MS = 45_000;
const IDLE_TIMEOUT_MS = 5 * 60 * 1000;
// Slightly above TICK_INTERVAL_MS so a briefly-throttled background tab
// (browser timer coalescing) doesn't lose a legitimate near-45s stretch, but
// far enough below the idle timeout that a single tick can never look like
// a whole idle/hidden period's worth of reading.
const MAX_REPORTABLE_SECONDS = 68;

// Events treated as "the reader is being actively used". Listened for at the
// document level with capture:true so they're seen regardless of where in
// the reader DOM they originate (scroll in particular does not bubble, so a
// non-capturing document listener would miss it).
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

/**
 * Reading-time heartbeat for the reader view: reports elapsed active seconds
 * to the backend every TICK_INTERVAL_MS so the dashboard heatmap/streak have
 * data to show.
 *
 * Lifecycle is managed explicitly by the caller (ReaderView.vue calls
 * start()/stop() from its own onMounted/onBeforeUnmount) rather than this
 * composable registering its own hooks, so it composes cleanly alongside
 * ReaderView's existing document-level keydown listener.
 *
 * Pausing (tab hidden, or 5 minutes without scroll/keydown/pointerdown)
 * stops the clock without sending a report; resuming resets the timing
 * origin to "now" rather than backfilling the paused duration, so hidden or
 * idle time is never counted as reading time. A final partial tick is
 * flushed on stop() (e.g. unmount) if the reader was active at that point.
 */
export function useReadingHeartbeat(bookID: () => string) {
  let intervalId: ReturnType<typeof setInterval> | null = null;
  let idleTimer: ReturnType<typeof setTimeout> | null = null;
  // Timestamp (ms) marking the start of the current not-yet-reported active
  // stretch. Reset after every successful report and whenever we resume
  // from a paused (hidden/idle) state.
  let tickAnchor = 0;
  let hidden = false;
  let idle = false;
  // Book ID latched at the start of the current accumulation stretch. Reports
  // always use this value (not the live getter): Vue Router reuses ReaderView
  // for /reader/:id param changes, so by the time a tick fires the getter may
  // already point at the next book while the elapsed seconds belong to the
  // previous one.
  let currentBookID = '';

  function report(seconds: number): void {
    const rounded = clampSeconds(seconds);
    if (rounded <= 0 || currentBookID === '') {
      return;
    }

    const date = toIsoDate(new Date());
    getBookshelfProvider()
      .reportReadingActivity(currentBookID, rounded, date)
      .catch((err) => {
        console.warn('Failed to report reading activity', err);
      });
  }

  // Flush the stretch accumulated for the previous book under its own ID,
  // then start a fresh stretch attributed to the new book.
  watch(bookID, (newBookID) => {
    if (intervalId !== null && !hidden && !idle) {
      const now = Date.now();
      report((now - tickAnchor) / 1000);
    }
    tickAnchor = Date.now();
    currentBookID = newBookID;
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
      tickAnchor = Date.now();
    }
    resetIdleTimer();
  }

  function onVisibilityChange(): void {
    if (document.hidden) {
      hidden = true;
      return;
    }
    hidden = false;
    tickAnchor = Date.now();
  }

  function tick(): void {
    if (hidden || idle) {
      return;
    }
    const now = Date.now();
    const elapsedSeconds = (now - tickAnchor) / 1000;
    tickAnchor = now;
    report(elapsedSeconds);
  }

  function start(): void {
    currentBookID = bookID();
    tickAnchor = Date.now();
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
      const elapsedSeconds = (Date.now() - tickAnchor) / 1000;
      report(elapsedSeconds);
    }
  }

  return { start, stop };
}
