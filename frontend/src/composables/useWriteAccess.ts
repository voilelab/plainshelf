import { computed } from 'vue';

import { useServerMode } from '@/composables/useServerMode';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { getBookshelfProvider, isWritableProvider } from '@/providers';

/**
 * Why write operations are unavailable, in precedence order. `platform` wins
 * over the two server-side reasons so user-facing copy names the reason the
 * user can actually act on, and `server-read-only` wins over
 * `shelf-read-only` because a read-only server opens every shelf read-only —
 * naming the shelf there would send the user to fix the narrower of the two
 * settings.
 */
type WriteDisabledReason = 'platform' | 'server-read-only' | 'shelf-read-only' | null;

/**
 * Whether this client can mutate the shelf at all.
 *
 * Asks the active provider rather than the runtime: a reading client is one
 * whose provider does not implement the write surface, which is what
 * `bookshelfWriter()` refuses on. Deliberately separate from the server's
 * `read_only` config and from the shelf's own — all three can be true at once,
 * and each means something different to the user.
 */
export function isLibraryEditingSupported(): boolean {
  return isWritableProvider(getBookshelfProvider());
}

/**
 * Write access plus the named platform capabilities that depend on it.
 *
 * The capability flags below answer "what can a reading client not do", in one
 * place rather than in each component. They are deliberately not interchangeable
 * with `isMobileRuntime()`, which stays the tool for mobile UX branching — how a
 * screen behaves, not what the client is allowed to do.
 *
 * The three libraryEditing* flags are separate from `writesEnabled` on purpose:
 * a read-only server, or a read-only shelf, still shows those views, because
 * the lists themselves are useful and the settings are still worth reading.
 */
export function useWriteAccess() {
  const { readOnly } = useServerMode();
  // Folded in here rather than checked at each write affordance, so a shelf
  // opened read-only withdraws the whole write surface in one place instead of
  // each component growing a condition one of them then forgets.
  const { selectedShelfReadOnly } = useShelvesStore();

  // isLibraryEditingSupported() is called inside the computed rather than
  // hoisted: the provider is created lazily on first use, so reading it at
  // module scope could resolve before the shell has finished configuring it.
  const writesEnabled = computed(
    () => !readOnly.value && !selectedShelfReadOnly.value && isLibraryEditingSupported()
  );

  const writeDisabledReason = computed<WriteDisabledReason>(() => {
    if (!isLibraryEditingSupported()) {
      return 'platform';
    }
    if (readOnly.value) {
      return 'server-read-only';
    }
    if (selectedShelfReadOnly.value) {
      return 'shelf-read-only';
    }
    return null;
  });

  // Copying *out of* a shelf only reads it: the write lands on the target, and
  // the server refuses a read-only source for a move alone, since a move ends by
  // deleting the original. So the transfer entry survives a read-only shelf and
  // the modals drop the move mode instead of hiding it.
  const outgoingCopyEnabled = computed(() => !readOnly.value && isLibraryEditingSupported());

  // The message a refused write reports, so a shelf-level refusal does not
  // blame the server the user would then find writable. A key rather than a
  // string: the caller translates it with its own `t`, which follows a locale
  // change this composable would not see.
  const writeDisabledMessageKey = computed(() =>
    writeDisabledReason.value === 'shelf-read-only'
      ? ('layout.readOnly.shelfWriteDisabled' as const)
      : ('layout.readOnly.writeDisabled' as const)
  );

  // Trash and the maintenance views exist only to fix up the library, so they
  // are hidden on the platform that cannot write.
  const libraryEditingAvailable = computed(() => isLibraryEditingSupported());

  // The cover, reader and import settings tabs POST to /api/setting/*, which the
  // read-only mobile client cannot do; the read-history tab writes device-local
  // state and stays everywhere. A read-only *shelf* does not touch these at all:
  // they are server-wide and outlive whichever shelf is browsed.
  const serverSettingsEditable = computed(() => isLibraryEditingSupported());

  // The logs view is unreachable on mobile (features/mobile/utils/blockedRoutes.ts);
  // this hides its nav entries to match. Reading the logs is not a write.
  const serverAdminAvailable = computed(() => isLibraryEditingSupported());

  return {
    writesEnabled,
    writeDisabledReason,
    writeDisabledMessageKey,
    outgoingCopyEnabled,
    libraryEditingAvailable,
    serverSettingsEditable,
    serverAdminAvailable
  };
}
