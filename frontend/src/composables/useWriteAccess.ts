import { computed } from 'vue';

import { useServerMode } from '@/composables/useServerMode';
import { isMobileRuntime } from '@/providers/runtime';

/**
 * Why write operations are unavailable, in precedence order. `platform` wins
 * over `server-read-only` so user-facing copy names the reason the user can
 * actually act on.
 */
export type WriteDisabledReason = 'platform' | 'server-read-only' | null;

/**
 * The Android client is a reading client: it browses, reads, downloads for
 * offline use, and records reading progress, but never mutates the shelf.
 *
 * This is a platform capability, deliberately separate from the server's
 * `read_only` config (see useServerMode). Both can be true at once, and the two
 * carry different user-facing meanings.
 */
export function isLibraryEditingSupported(): boolean {
  return !isMobileRuntime();
}

export function useWriteAccess() {
  const { readOnly } = useServerMode();

  // isLibraryEditingSupported() is called inside the computed rather than
  // hoisted, matching the isMobileEnv pattern elsewhere: the runtime never
  // changes mid-session, but the ?mobile-shell-preview=1 escape hatch is read
  // from the URL.
  const writesEnabled = computed(() => !readOnly.value && isLibraryEditingSupported());

  const writeDisabledReason = computed<WriteDisabledReason>(() => {
    if (!isLibraryEditingSupported()) {
      return 'platform';
    }
    if (readOnly.value) {
      return 'server-read-only';
    }
    return null;
  });

  return { writesEnabled, writeDisabledReason };
}
