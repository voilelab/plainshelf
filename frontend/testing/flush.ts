/**
 * Waiting for the DOM to catch up, in the three shapes this suite actually
 * needs. They were 25 local definitions across eight bodies before this file,
 * which is 25 chances to pick the wrong one and a test that passes for a reason
 * nobody can name.
 *
 * Reach for the weakest one that works. A stronger wait hides a missing `await`
 * in the code under test rather than failing on it.
 */
import { nextTick } from 'vue';

/** A microtask: enough for a promise chain that touches no timer. */
export function flushMicrotasks(): Promise<void> {
  return Promise.resolve().then(() => undefined);
}

/** One macrotask: enough for a `setTimeout(fn, 0)` the code under test queued. */
export function flushMacrotask(): Promise<void> {
  return new Promise((done) => setTimeout(done));
}

/**
 * Two macrotasks plus a render: the wait for a chain that resolves a promise,
 * queues a timeout from its `then`, and renders the result. This is what most
 * of the local `flush()` copies were.
 */
export async function flush(): Promise<void> {
  await flushMacrotask();
  await flushMacrotask();
  await nextTick();
}
