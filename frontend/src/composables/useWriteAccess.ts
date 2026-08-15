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

/**
 * Write access plus the named platform capabilities that depend on it.
 *
 * The capability flags below answer "what does the Android client not have",
 * and they live here rather than in each component so that question has one
 * answer. They are deliberately *not* interchangeable with a plain mobile-runtime
 * check: `isMobileRuntime()` stays the right tool for mobile UX branching —
 * the Downloads nav entry, tap-to-select in the book grid, the Android back
 * button — which is about how a screen behaves, not about what the platform
 * is allowed to do.
 */
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

  // Trash and the maintenance views exist only to fix up the library, so they
  // are hidden on the platform that cannot write. Kept separate from
  // `writesEnabled`: a read-only server still shows them, since the lists
  // themselves are useful.
  const libraryEditingAvailable = computed(() => isLibraryEditingSupported());

  // The cover, reader, and import settings tabs POST to /api/setting/*, which
  // the read-only mobile client cannot do. The read-history tab only writes
  // device-local state and stays available everywhere. Separate from
  // `writesEnabled` for the same reason as above: these are server-wide
  // settings, and a read-only server still renders them read-only rather than
  // dropping the tabs.
  const serverSettingsEditable = computed(() => isLibraryEditingSupported());

  // Server administration is not part of a reading client, so the logs view is
  // unreachable on mobile (see features/mobile/utils/blockedRoutes.ts). This
  // hides its nav entries to match. Not folded into the two flags above: a
  // read-only server still administers itself, and reading the logs is not a
  // write.
  const serverAdminAvailable = computed(() => isLibraryEditingSupported());

  return {
    writesEnabled,
    writeDisabledReason,
    libraryEditingAvailable,
    serverSettingsEditable,
    serverAdminAvailable
  };
}
