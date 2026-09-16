/**
 * One way to mount a component in a test.
 *
 * The suite grew 51 hand-rolled `createApp(...).mount(host)` helpers because
 * there was no shared one: this project has no `@vue/test-utils`, and adding a
 * dependency for four lines of setup was never worth it. Four lines repeated
 * fifty times is worth something else, though — most of the copies differ only
 * in the component's name, and a few differ in ways nobody intended. The worst
 * of those is a teardown the *test body* has to opt into (`mounted = app;`
 * after every mount): forget it in one case and that case leaks its app into
 * the next one, and nothing says so.
 *
 * So this module owns the registry and `testing/setup.ts` drains it after every
 * test, which is the part a file can no longer get wrong. It deliberately does
 * not own anything else: captured emits, component stubs and provider mocks are
 * per-component and stay in the file that needs them.
 *
 * It lives outside `src/` on purpose. `scripts/check-exports.mjs` counts an
 * export that only test files name as unused, and nothing here should be able
 * to reach the bundle in the first place.
 *
 * Seventeen files use it so far and thirty-four still hand-roll their own. That
 * is deliberate rather than unfinished: the ones converted were the ones where
 * this helper is the same thing written once. A file stays hand-rolled when its
 * mount is part of what it tests —
 *
 *   - it mounts and unmounts inside one case, to model a reload or to assert
 *     what an in-flight request does when the component goes away
 *     (useSidebarLayout, useDashboardData, BookDetail);
 *   - it keeps two apps alive at once (LogRetentionPanel's capped case);
 *   - it installs a global its own teardown must remove (ReaderBlockWindow's
 *     IntersectionObserver, MobileReaderView's fake timers);
 *   - it mounts a RouterView with the subject as a route record, rather than
 *     mounting the subject (useUnsavedChangesGuard);
 *   - or it renders something other than a component — a VNode factory
 *     (localizedChrome), or `renderToString` (bookSummary).
 *
 * Convert the rest as you touch them. Do not convert one of the above to make
 * the count go up.
 */
import { createApp, defineComponent, h, type App, type Component } from 'vue';

export interface MountOptions {
  /**
   * Props handed to the component, including `onEventName` listeners. Pass a
   * function when a test mutates what it hands in — a `v-model` host, or a
   * parent prop a case changes mid-test. It is read on every render, so the
   * component sees the new value; a plain object is read once, which is what a
   * static prop wants and what silently breaks a two-way binding.
   */
  props?: Record<string, unknown> | (() => Record<string, unknown>);
  /** Slots, as Vue's own render-function slot object. */
  slots?: Record<string, (...args: never[]) => unknown>;
  /** Globally registered components, for stubbing `RouterLink` and friends. */
  components?: Record<string, Component>;
  /** Plugins to install, a router being the usual one. */
  plugins?: Parameters<App['use']>[0][];
  /**
   * Attach the host to `document.body`. On by default: a detached tree has no
   * `activeElement`, so focus assertions on one pass vacuously. Pass false only
   * when a test wants two trees that cannot see each other.
   */
  attach?: boolean;
}

export interface Mounted {
  host: HTMLElement;
  app: App;
  /** Unmounts now, for a test that is *about* unmounting. Idempotent. */
  unmount(): void;
}

const mounted: Mounted[] = [];

export function mount(component: Component, options: MountOptions = {}): Mounted {
  const { props, slots, components, plugins, attach = true } = options;

  const host = document.createElement('div');
  if (attach) {
    document.body.append(host);
  }

  // The component is rendered through a wrapper rather than passed to
  // createApp directly, so slots work the same way props do and a caller never
  // has to know which of the two forms a given component needs.
  const readProps = typeof props === 'function' ? props : () => props;
  const app = createApp(defineComponent({ setup: () => () => h(component, readProps(), slots) }));
  for (const plugin of plugins ?? []) {
    app.use(plugin);
  }
  for (const [name, stub] of Object.entries(components ?? {})) {
    app.component(name, stub);
  }
  app.mount(host);

  let live = true;
  const entry: Mounted = {
    host,
    app,
    unmount() {
      if (!live) {
        return;
      }
      live = false;
      app.unmount();
      host.remove();
    }
  };
  mounted.push(entry);
  return entry;
}

/** Unmounts everything `mount` created, newest first. Called by the setup file. */
export function unmountAll(): void {
  for (const entry of mounted.splice(0).reverse()) {
    entry.unmount();
  }
}
