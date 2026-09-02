import { computed } from 'vue';

import { useServerMode } from '@/composables/useServerMode';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { getBookshelfProvider, isWritableProvider } from '@/providers';
import { t } from '@/i18n';

/**
 * Why write operations are unavailable, in precedence order: `platform`, then
 * `server-read-only`, then `shelf-read-only`. The order runs from the widest
 * cause to the narrowest, so user-facing copy names the one the user would have
 * to lift first.
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
 * useServerMode) and from the active shelf's own `read_only` (see
 * useShelvesStore). Any of them can be true at once, and each carries a
 * different user-facing meaning.
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
  // The active shelf's own read_only, which a writable server can still report
  // for one of its shelves. Folded in here rather than checked per component so
  // every write entry already gated on `writesEnabled` picks it up.
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
  // alone (server/handle_book_transfers.go). So the transfer entry survives a
  // read-only shelf, and the modals drop the move mode instead.
  const outgoingCopyEnabled = computed(() => !readOnly.value && isLibraryEditingSupported());

  // Why a write just refused was refused, for the guards that report it inline
  // rather than by hiding the entry. A function rather than a computed so the
  // string is resolved at the moment it is shown, in the locale current then.
  function writeDisabledMessage(): string {
    return writeDisabledReason.value === 'shelf-read-only'
      ? t('layout.readOnly.shelfWriteDisabled')
      : t('layout.readOnly.writeDisabled');
  }

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
    writeDisabledMessage,
    outgoingCopyEnabled,
    libraryEditingAvailable,
    serverSettingsEditable,
    serverAdminAvailable
  };
}
