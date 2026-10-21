import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { useRouter } from 'vue-router';
import { useToasts } from '@/composables/useToasts';
import { getReaderLaunchMode } from '@/composables/useReaderLaunchPreference';
import { isReaderUnsupportedPlatform } from '@/api/desktop';
import { isWebRuntime } from '@/providers';
import { t } from '@/i18n';

export function readerRoutePath(id: string): string {
  return `/reader/${id}`;
}

export function useReaderLaunch() {
  const router = useRouter();
  const { showToast } = useToasts();

  function launchReader(id: string, sectionIndex?: number): void {
    const hasSection = typeof sectionIndex === 'number' && Number.isFinite(sectionIndex);
    const to = hasSection
      ? { path: readerRoutePath(id), query: { section: String(Math.trunc(sectionIndex)) } }
      : { path: readerRoutePath(id) };

    const openInNewReader = getReaderLaunchMode() === 'new-reader';

    // HTML Standard window.open returns null with noopener even when opening succeeds.
    if (isWebRuntime()) {
      if (openInNewReader) {
        window.open(router.resolve(to).href, '_blank', 'noopener,noreferrer');
        return;
      }
      void router.push(to);
      return;
    }

    const platform = getPlatformCapabilities();
    if (platform.openReader && openInNewReader) {
      platform.openReader(id, hasSection ? Math.trunc(sectionIndex) : undefined).catch((err) => {
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
