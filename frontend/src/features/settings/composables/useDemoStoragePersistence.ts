import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue';

// Set only by the wasm demo's boot.js (cmd/plainshelf-wasm/web), so its
// presence is what tells the browser-only demo from a server, desktop or app.
interface PlainshelfDemo {
  persistEvent: string;
  hasStorage(): Promise<boolean>;
  canPersist(): boolean;
  persisted(): Promise<boolean>;
  persist(): Promise<boolean>;
}

declare global {
  interface Window {
    plainshelfDemo?: PlainshelfDemo;
  }
}

function demo(): PlainshelfDemo | undefined {
  return typeof window === 'undefined' ? undefined : window.plainshelfDemo;
}

export function isWasmDemo(): boolean {
  return demo() !== undefined;
}

/**
 * - `memory`: the demo has no storage at all, so there is nothing to keep
 * - `unsupported`: the browser cannot be asked
 * - `persisted`: the browser agreed not to clear it on its own
 * - `best-effort`: the browser may clear it when space runs low
 */
type DemoStorageState = 'loading' | 'memory' | 'unsupported' | 'persisted' | 'best-effort';

export function useDemoStoragePersistence(): {
  state: Ref<DemoStorageState>;
  requesting: Ref<boolean>;
  declined: Ref<boolean>;
  request: () => Promise<void>;
} {
  const state = ref<DemoStorageState>('loading');
  const requesting = ref(false);
  const declined = ref(false);

  async function refresh(): Promise<void> {
    const api = demo();
    if (!api) return;
    if (!(await api.hasStorage())) state.value = 'memory';
    else if (!api.canPersist()) state.value = 'unsupported';
    else state.value = (await api.persisted()) ? 'persisted' : 'best-effort';
  }

  async function request(): Promise<void> {
    const api = demo();
    if (!api || requesting.value) return;
    requesting.value = true;
    try {
      declined.value = !(await api.persist());
    } catch {
      declined.value = true;
    } finally {
      requesting.value = false;
    }
    await refresh();
  }

  // The Demo bar asks too; follow its answer.
  const onPersist = () => void refresh();
  onMounted(() => {
    const api = demo();
    if (!api) return;
    window.addEventListener(api.persistEvent, onPersist);
    void refresh();
  });
  onBeforeUnmount(() => {
    const api = demo();
    if (api) window.removeEventListener(api.persistEvent, onPersist);
  });

  return { state, requesting, declined, request };
}
