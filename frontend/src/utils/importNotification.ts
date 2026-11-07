import { t } from '@/i18n';
import { getPlatformCapabilities } from '@/providers/platformCapabilities';

type ImportedItems = 'books' | 'shelves';

export function notifyImportFinished(items: ImportedItems, imported: number, failed: number): void {
  const notify = getPlatformCapabilities().notifyImportFinished;
  if (!notify) {
    return;
  }
  const variant = failed > 0 ? `${items}WithFailures` : items;
  const body = t(`importNotification.${variant}`, { count: imported, failed });
  void notify(t('importNotification.title'), body).catch((err: unknown) => {
    console.error('Failed to send import notification:', err);
  });
}
