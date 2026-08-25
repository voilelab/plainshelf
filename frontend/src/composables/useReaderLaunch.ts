import { useRouter } from 'vue-router';
import { useToasts } from '@/composables/useToasts';
import { getReaderLaunchMode } from '@/composables/useReaderLaunchPreference';
import { isReaderUnsupportedPlatform } from '@/api/desktop';
import { getBookshelfProvider, isWebRuntime } from '@/providers';
import { t } from '@/i18n';

/**
 * The in-app reader route for a book. This is the single place the `/reader/:id`
 * path is spelled outside the router: every reading entry that needs a real
 * `href` — the home cards render a RouterLink `custom` slot so a middle-click or
 * "open in new tab" still works — builds its target from here rather than
 * writing the path itself. That is what lets `check-module-boundaries`'s
 * reader-entrypoint rule hold the launch policy to one entrance; a card that
 * spells `/reader/…` on its own would silently bypass the preference again.
 */
export function readerRoutePath(id: string): string {
  return `/reader/${id}`;
}

/**
 * Opening a book in the reader, honouring the device-local "reader launch
 * preference". This is the single launch path shared by every reading entry —
 * the library, book detail and reading history through `useBookActions.goRead`,
 * and the home dashboard's "recent reading" / "read now" cards directly.
 *
 * It deliberately depends on nothing but `useRouter` and `useToasts` (the rest
 * are module-level helpers, not stores), so the home components can call it
 * without pulling in the book/folder stores `useBookActions` carries.
 */
export function useReaderLaunch() {
  const router = useRouter();
  const { showToast } = useToasts();

  /**
   * Opens the reader, optionally at one chapter instead of the saved progress.
   * The index is the reader's own section index, so it survives a title change.
   */
  function launchReader(id: string, sectionIndex?: number): void {
    const hasSection = typeof sectionIndex === 'number' && Number.isFinite(sectionIndex);
    const to = hasSection
      ? { path: readerRoutePath(id), query: { section: String(Math.trunc(sectionIndex)) } }
      : { path: readerRoutePath(id) };

    // Device-local preference: 'new-reader' (default) opens a fresh reader,
    // 'in-window' navigates the current window in place. Only the web and
    // desktop shells have a fresh-reader path to gate; the mobile and standalone
    // reader shells always navigate in place regardless — isWebRuntime() is
    // false for them and they define no openDesktopReader, so they fall straight
    // through to the router.push at the end, unaffected by the preference.
    const openInNewReader = getReaderLaunchMode() === 'new-reader';

    // On a plain web-server build, opening a new reader means a new tab so the
    // library or book page it was launched from is not replaced wholesale —
    // closing the tab returns the reader there. Matches externalLinks.ts's
    // window.open idiom.
    //
    // No router.push fallback on the window.open itself: with noopener/noreferrer
    // window.open returns null even on success (per the HTML spec), so its result
    // cannot tell a blocked pop-up from an opened one. Pushing "on failure" would
    // therefore fire on every success too and navigate the original tab as well,
    // which is exactly the wholesale replacement this feature avoids. A
    // user-gesture open is not pop-up-blocked in practice. When the preference is
    // 'in-window', we deliberately router.push instead — that is the requested
    // in-place navigation, not a fallback.
    if (isWebRuntime()) {
      if (openInNewReader) {
        window.open(router.resolve(to).href, '_blank', 'noopener,noreferrer');
        return;
      }
      void router.push(to);
      return;
    }

    // On the desktop app, opening a new reader means the standalone reader in its
    // own window; its progress flows back into this library (reading_progress.json).
    // Only the desktop provider defines openDesktopReader, so web/mobile fall
    // through. A chapter jump passes its section so the reader opens on that
    // chapter — the same window as the default read, not the in-app one — and the
    // in-app /reader/:id route stays the fallback if the reader will not launch
    // (e.g. not installed). When the preference is 'in-window', skip the shell-out
    // entirely and navigate in place.
    const provider = getBookshelfProvider();
    if (provider.openDesktopReader && openInNewReader) {
      provider.openDesktopReader(id, hasSection ? Math.trunc(sectionIndex) : undefined).catch((err) => {
        // The standalone reader did not launch, so fall back to the in-app
        // reader as before. But the user explicitly picked "open a new reader",
        // so a silent fallback reads as "my setting is broken" — explain instead
        // why the book opened here. Two shapes: this platform has no standalone
        // reader at all (non-macOS), or the macOS reader failed to launch (not
        // installed, or the launch itself errored).
        void router.push(to);
        showToast(
          isReaderUnsupportedPlatform(err)
            ? t('bookDetail.messages.readerUnsupportedPlatform')
            : t('bookDetail.messages.readerLaunchFailed')
        );
      });
      return;
    }

    void router.push(to);
  }

  /**
   * Click handler for a reading entry rendered as a real `<a href>` (a
   * RouterLink `custom` slot). A plain left click runs the launch preference;
   * a modified click (⌘/Ctrl/Shift/Alt) or any non-primary button is the user
   * asking the browser to open the URL its own way — a new tab or window — so it
   * is left to the browser default and the preference is bypassed, exactly as a
   * normal link behaves. Non-primary buttons fire `auxclick`, not `click`, so
   * they never reach here; the `button` guard is belt-and-suspenders.
   */
  function onReaderLinkClick(event: MouseEvent, id: string, sectionIndex?: number): void {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    event.preventDefault();
    launchReader(id, sectionIndex);
  }

  return { launchReader, onReaderLinkClick };
}
