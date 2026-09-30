import { useRouter } from 'vue-router';
import { useToasts } from '@/composables/useToasts';
import { getReaderLaunchMode } from '@/composables/useReaderLaunchPreference';
import { isReaderUnsupportedPlatform } from '@/api/desktop';
import { getBookshelfProvider, isWebRuntime } from '@/providers';
import { t } from '@/i18n';

/**
 * The single place `/reader/:id` is spelled outside the router: every reading
 * entry that needs a real `href` builds its target here. That is what lets
 * `check-module-boundaries`'s reader-entrypoint rule hold the launch policy to
 * one entrance — a card that spelled the path itself would silently bypass the
 * preference again.
 */
export function readerRoutePath(id: string): string {
  return `/reader/${id}`;
}

/**
 * The single launch path shared by every reading entry, honouring the
 * device-local "reader launch preference".
 *
 * It deliberately depends on nothing but `useRouter` and `useToasts`, so the
 * home components can call it without pulling in the book and folder stores
 * `useBookActions` carries.
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

    // 'new-reader' (the default) opens a fresh reader, 'in-window' navigates in
    // place. Only web and desktop have a fresh-reader path to gate: mobile and
    // the standalone reader fall straight through to the router.push at the end.
    const openInNewReader = getReaderLaunchMode() === 'new-reader';

    // On the web, a new reader is a new tab, so the page it was launched from is
    // not replaced wholesale.
    //
    // No router.push fallback on the window.open: with noopener/noreferrer it
    // returns null even on success (per the HTML spec), so "on failure" would
    // fire on every success too and navigate the original tab as well — exactly
    // the replacement this avoids. The router.push under 'in-window' is the
    // requested navigation, not a fallback.
    if (isWebRuntime()) {
      if (openInNewReader) {
        window.open(router.resolve(to).href, '_blank', 'noopener,noreferrer');
        return;
      }
      void router.push(to);
      return;
    }

    // On desktop, a new reader is the standalone app in its own window, whose
    // progress flows back through reading_progress.json. Only the desktop
    // provider defines openDesktopReader, so web and mobile fall through, and the
    // in-app route stays the fallback when the reader will not launch.
    const provider = getBookshelfProvider();
    if (provider.openDesktopReader && openInNewReader) {
      provider.openDesktopReader(id, hasSection ? Math.trunc(sectionIndex) : undefined).catch((err) => {
        // Fall back to the in-app reader, but say why: the user explicitly asked
        // for a new reader, so a silent fallback reads as "my setting is broken".
        // Either this platform has no standalone reader, or it failed to launch.
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
   * For a reading entry rendered as a real `<a href>`. A plain left click runs
   * the launch preference; a modified click is the user asking the browser to
   * open the URL its own way, so it is left to the browser default exactly as a
   * normal link behaves. Non-primary buttons fire `auxclick`, so the `button`
   * guard is belt-and-suspenders.
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
