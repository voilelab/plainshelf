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
    estimate?: (item: TItem, index: number) => number;
    overscan?: number;
  }>(),
  { estimate: undefined, overscan: 800 }
);

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
    // Vue 3.5.43 refreshes callback refs during vnode patching (runtime-core setRef).
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
