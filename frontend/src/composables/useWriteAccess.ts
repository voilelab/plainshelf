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
 * whose provider does not implement the write surface, which is the same thing
 * `bookshelfWriter()` refuses on. Today that is the Android shell and the
 * pCloud backend — both browse, read, download for offline use and record
 * reading progress, but never mutate the shelf — while the server and desktop
 * providers are writable.
 *
 * Deliberately separate from the server's `read_only` config (see
 * useServerMode) and from the shelf's own (see useShelvesStore). All three can
 * be true at once, and each carries a different user-facing meaning.
 */
export function isLibraryEditingSupported(): boolean {
  return isWritableProvider(getBookshelfProvider());
}

/**
 * Write access plus the named platform capabilities that depend on it.
 *
 * The capability flags below answer "what can a reading client not do", and
 * they live here rather than in each component so that question has one answer.
 * They are deliberately *not* interchangeable with a runtime check:
 * `isMobileRuntime()` stays the right tool for mobile UX branching — tap-to-
 * select in the book grid, the Android back button — which is about how a
 * screen behaves, not about what the client is allowed to do.
 */
export function useWriteAccess() {
  const { readOnly } = useServerMode();
  // The per-shelf flag is folded in here rather than checked at each write
  // affordance: every one of them already asks this composable, so a shelf
  // opened read-only withdraws the whole write surface in one place instead of
  // each component growing its own condition and one of them being forgotten.
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

  // Copying a book or folder *out of* a shelf only reads that shelf: the write
  // lands on the target, and the server refuses a read-only source for a move
  // alone, because a move ends by deleting the original
  // (shelf/shelf_cross_book_test.go). So the transfer entry survives a
  // read-only shelf and the modals drop the move mode instead of hiding it.
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
  // are hidden on the platform that cannot write. Kept separate from
  // `writesEnabled`: a read-only server — or a read-only shelf — still shows
  // them, since the lists themselves are useful.
  const libraryEditingAvailable = computed(() => isLibraryEditingSupported());

  // The cover, reader, and import settings tabs POST to /api/setting/*, which
  // the read-only mobile client cannot do. The read-history tab only writes
  // device-local state and stays available everywhere. Separate from
  // `writesEnabled` for the same reason as above: these are server-wide
  // settings, and a read-only server still renders them read-only rather than
  // dropping the tabs. A read-only *shelf* does not touch them at all: they are
  // server-wide and outlive whichever shelf is being browsed.
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
    writeDisabledMessageKey,
    outgoingCopyEnabled,
    libraryEditingAvailable,
    serverSettingsEditable,
    serverAdminAvailable
  };
}
