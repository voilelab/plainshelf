<template>
  <div ref="rootEl" class="reader-block-window">
    <div
      v-for="(item, index) in items"
      :key="index"
      :ref="(el) => setWrapper(index, el)"
      :data-window-index="index"
      class="reader-block-window-item"
      :style="active[index] ? undefined : { minHeight: `${heights[index]}px` }"
    >
      <slot v-if="active[index]" :item="item" :index="index" />
    </div>
  </div>
</template>

<script setup lang="ts" generic="TItem">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';

const props = withDefaults(
  defineProps<{
    items: readonly TItem[];
    /** Estimated pixel height for a block that has not been measured yet. */
    estimate?: (item: TItem, index: number) => number;
    /** Extra viewport margin, in CSS pixels, kept mounted above and below. */
    overscan?: number;
  }>(),
  { estimate: undefined, overscan: 800 }
);

// Off-screen blocks collapse to a placeholder of their last measured height so
// the scroll container keeps its full height. useReader's scroll<->offset bridge
// derives the char offset from scrollHeight, so preserving it is what keeps
// virtualization from shifting a saved reading position (see the ticket).
const active = ref<boolean[]>([]);
const heights = ref<number[]>([]);

const rootEl = ref<HTMLDivElement | null>(null);
const wrappers = new Map<number, HTMLElement>();
let observer: IntersectionObserver | null = null;

const supportsObserver = typeof IntersectionObserver !== 'undefined';

function estimateFor(index: number): number {
  const item = props.items[index];
  if (item === undefined) return 0;
  const value = props.estimate?.(item, index);
  return Number.isFinite(value) && (value as number) > 0 ? (value as number) : 320;
}

function measure(index: number): void {
  const el = wrappers.get(index);
  if (el && el.offsetHeight > 0) {
    heights.value[index] = el.offsetHeight;
  }
}

function reset(): void {
  const count = props.items.length;
  // Without an observer (jsdom, or a webview that lacks it) everything mounts,
  // which is exactly the reader's pre-virtualization behavior. With one, the
  // first block starts mounted so reading from the top paints immediately;
  // the observer resolves the rest (including a deep resume) on its first pass.
  active.value = Array.from({ length: count }, (_unused, index) => !supportsObserver || index === 0);
  heights.value = Array.from({ length: count }, (_unused, index) => estimateFor(index));
}

function onIntersections(entries: IntersectionObserverEntry[]): void {
  for (const entry of entries) {
    const index = Number((entry.target as HTMLElement).dataset.windowIndex);
    if (!Number.isInteger(index)) continue;
    if (entry.isIntersecting) {
      if (!active.value[index]) {
        active.value[index] = true;
        void nextTick(() => measure(index));
      }
    } else if (active.value[index]) {
      // Record the real height before collapsing so the placeholder holds it.
      measure(index);
      active.value[index] = false;
    }
  }
}

function setWrapper(index: number, el: unknown): void {
  const element = el instanceof HTMLElement ? el : null;
  const previous = wrappers.get(index);
  if (previous && previous !== element && observer) {
    observer.unobserve(previous);
  }
  if (element) {
    wrappers.set(index, element);
    if (observer) observer.observe(element);
  } else if (previous) {
    wrappers.delete(index);
  }
}

function connect(): void {
  if (!supportsObserver) return;
  observer = new IntersectionObserver(onIntersections, {
    rootMargin: `${props.overscan}px 0px`
  });
  for (const el of wrappers.values()) {
    observer.observe(el);
  }
}

function disconnect(): void {
  observer?.disconnect();
  observer = null;
}

reset();

onMounted(connect);
onBeforeUnmount(disconnect);

watch(
  () => props.items,
  () => {
    disconnect();
    wrappers.clear();
    reset();
    // The v-for re-runs its element refs on the next flush; reconnect after.
    void nextTick(connect);
  }
);
</script>

<style scoped>
.reader-block-window-item {
  /* A plain block wrapper: it must not introduce spacing of its own, so the
     reader's block margins stay owned by the rendered content inside. */
  display: block;
}
</style>
