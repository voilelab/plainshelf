<template>
  <section class="detail-shell">
    <div class="detail-container">
      <DeleteModal
        :open="!!deleteTarget"
        :item-name="deleteTarget?.title || id"
        :description="t('bookDetail.delete.description')"
        :busy="deleting"
        @cancel="cancelDelete"
        @confirm="confirmDelete"
      />

      <div v-if="showImportedMessage" class="loading detail-notice" role="status">
        {{ t('bookDetail.messages.imported') }}
      </div>
      <div v-if="showSavedMessage" class="loading detail-notice" role="status">
        {{ t('bookDetail.messages.saved') }}
      </div>
      <div v-if="actionError" class="error detail-error" role="alert">
        <p>{{ actionError }}</p>
        <button class="button" type="button" @click="dismissActionError">
          {{ t('bookDetail.actions.dismiss') }}
        </button>
      </div>
      <div v-if="loading" class="loading detail-notice">{{ t('bookDetail.loading') }}</div>
      <div v-else-if="error" class="error detail-error" role="alert">
        <p>{{ error }}</p>
        <button class="button" type="button" @click="fetchDetail">{{ t('common.retry') }}</button>
      </div>

      <article v-else-if="book" class="detail-panel">
        <div class="detail-cover-col">
          <BookCover
            :key="book.id"
            :book-id="book.id"
            :title="book.title"
            :authors="book.authors"
            :cover-url="book.cover_url"
            :read-only="readOnly"
            @cover-changed="onCoverChanged"
          />
        </div>

        <BookDetail :book="book" :progress="progress" :current-source="currentSource">
          <template #reading>
            <section class="reading-card" :aria-label="t('bookDetail.progress.sectionLabel')">
              <div class="progress-copy">
                <div>
                  <p class="progress-eyebrow">{{ t('bookDetail.progress.eyebrow') }}</p>
                  <p class="progress-status">{{ readingStatusLabel }}</p>
                </div>
                <strong class="progress-percent">{{ readingPercent }}%</strong>
              </div>
              <ProgressBar
                :value="readingPercent"
                :label="t('bookDetail.progress.label', { percent: readingPercent })"
              />

              <div class="primary-actions">
                <button
                  class="button primary read-button"
                  type="button"
                  :disabled="restartingRead"
                  @click="onReadClick"
                >
                  {{ readingActionLabel }}
                </button>
                <button class="button" type="button" :disabled="downloading" @click="downloadBook(book)">
                  {{ downloading ? t('bookDetail.actions.exporting') : t('bookDetail.actions.export') }}
                </button>
                <button
                  v-if="showOfflineDownloadButton"
                  class="button"
                  type="button"
                  :disabled="offlineDownloadDisabled"
                  @click="onOfflineDownloadClick"
                >
                  {{ offlineDownloadButtonLabel }}
                </button>

                <DropdownMenuRoot v-if="showManagementMenu">
                  <DropdownMenuTrigger class="button more-trigger" type="button">
                    {{ t('bookDetail.actions.more') }}
                    <span aria-hidden="true">···</span>
                  </DropdownMenuTrigger>
                  <DropdownMenuPortal>
                    <DropdownMenuContent class="reka-menu detail-actions-menu" align="end" :side-offset="6">
                      <DropdownMenuItem
                        v-if="!readOnly"
                        class="reka-menu-item"
                        @select="goEdit(id)"
                      >
                        {{ t('bookDetail.actions.editMetadata') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        v-if="!readOnly"
                        class="reka-menu-item"
                        @select="goEditSources"
                      >
                        {{ t('bookDetail.actions.editSources') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        v-if="!readOnly && currentSource"
                        class="reka-menu-item"
                        :disabled="refreshingStats"
                        @select="onRefreshStats"
                      >
                        {{ refreshingStats ? t('bookDetail.actions.updatingStats') : t('bookDetail.actions.updateStats') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        v-if="canOpenBookFolder"
                        class="reka-menu-item"
                        @select="openBookFolder(id)"
                      >
                        {{ t('bookDetail.actions.openFolder') }}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator v-if="!readOnly" class="detail-menu-separator" />
                      <DropdownMenuItem
                        v-if="!readOnly"
                        class="reka-menu-item detail-menu-danger"
                        :disabled="deleting"
                        @select="onRequestDelete"
                      >
                        {{ deleting ? t('bookDetail.actions.movingToTrash') : t('bookDetail.actions.moveToTrash') }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenuPortal>
                </DropdownMenuRoot>
              </div>

              <p v-if="offlineDownloadError" class="offline-download-error" role="alert">
                {{ offlineDownloadError }}
              </p>
            </section>
          </template>
        </BookDetail>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from 'reka-ui';
import BookCover from '@/features/library/components/BookCover.vue';
import BookDetail from '@/features/library/components/BookDetail.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import ProgressBar from '@/components/ProgressBar.vue';
import { useBookActions } from '@/composables/useBookActions';
import { useBookDetail } from '@/features/library/composables/useBookDetail';
import { getReadingAction, resolveReadingPercent } from '@/features/library/utils/bookDetail';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useOfflineDownload } from '@/composables/useOfflineDownload';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { bookshelfWriter, getBookshelfProvider, isMobileRuntime } from '@/providers';
import { booksRouteForLayerPath, getLayerPath } from '@/utils/layers';
import { useI18n } from '@/i18n';

const route = useRoute();
const router = useRouter();
const id = computed(() => String(route.params.id));
const showImportedMessage = computed(() => route.query.imported === '1');
const showSavedMessage = computed(() => route.query.saved === '1');
const { writesEnabled } = useWriteAccess();
const readOnly = computed(() => !writesEnabled.value);
const { t } = useI18n();

const {
  book,
  progress,
  progressContentLength,
  currentSource,
  loading,
  error,
  fetchDetail
} = useBookDetail(() => id.value);

const {
  downloading,
  actionError,
  deleteTarget,
  deleting,
  canOpenBookFolder,
  goRead,
  goEdit,
  openBookFolder,
  downloadBook,
  requestDelete,
  cancelDelete,
  confirmDelete,
  dismissActionError
} = useBookActions({
  onDeleted: (deleted) => {
    void router.push(booksRouteForLayerPath(getLayerPath(deleted)));
  }
});

const readingPercent = computed(() => resolveReadingPercent(
  progress.value,
  progressContentLength.value ?? undefined
));
const readingAction = computed(() => getReadingAction(readingPercent.value));
const readingActionLabel = computed(() => {
  if (readingAction.value === 'continue') {
    return t('bookDetail.actions.continueReading', { percent: readingPercent.value });
  }
  return t(`bookDetail.actions.${readingAction.value === 'reread' ? 'reread' : 'startReading'}`);
});
const readingStatusLabel = computed(() => t(`bookDetail.progress.${readingAction.value}`));
const showManagementMenu = computed(() => !readOnly.value || canOpenBookFolder.value);
const restartingRead = ref(false);

async function onReadClick(): Promise<void> {
  if (restartingRead.value) {
    return;
  }

  if (readingAction.value !== 'reread') {
    goRead(id.value);
    return;
  }

  restartingRead.value = true;
  actionError.value = '';
  try {
    await getBookshelfProvider().saveReadProgress(id.value, { char_offset: 0 });
    goRead(id.value);
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.restartReading');
  } finally {
    restartingRead.value = false;
  }
}

// Saving content to a file and keeping an offline device copy are deliberately
// separate actions. The labels in the template make that distinction explicit.
const {
  state: offlineDownloadState,
  error: offlineDownloadError,
  supported: offlineDownloadSupported,
  refresh: refreshOfflineDownload,
  download: startOfflineDownload,
  remove: removeOfflineDownload
} = useOfflineDownload(() => id.value);

const showOfflineDownloadButton = computed(() => isMobileRuntime() && offlineDownloadSupported.value);
const offlineDownloadDisabled = computed(() => offlineDownloadState.value === 'downloading');
const offlineDownloadButtonLabel = computed(() => {
  switch (offlineDownloadState.value) {
    case 'downloading':
      return t('downloads.detail.downloading');
    case 'downloaded':
    case 'update_available':
      return t('downloads.detail.remove');
    case 'failed':
      return t('downloads.detail.retry');
    default:
      return t('downloads.detail.download');
  }
});

function onOfflineDownloadClick(): void {
  if (offlineDownloadState.value === 'downloaded' || offlineDownloadState.value === 'update_available') {
    void removeOfflineDownload();
    return;
  }
  void startOfflineDownload();
}

const refreshingStats = ref(false);

async function onRefreshStats(): Promise<void> {
  const src = currentSource.value;
  if (!src) return;
  refreshingStats.value = true;
  actionError.value = '';
  try {
    await bookshelfWriter().refreshSourceMeta(id.value, src.id);
    await fetchDetail();
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.updateStats');
  } finally {
    refreshingStats.value = false;
  }
}

useDocumentTitle(() => [t('bookDetail.documentTitle'), book.value?.title, 'PlainShelf']);

function goEditSources(): void {
  if (readOnly.value) {
    return;
  }
  void router.push(`/books/${id.value}/sources`);
}

function onCoverChanged(): void {
  void fetchDetail();
}

function onRequestDelete(): void {
  if (readOnly.value || !book.value) {
    return;
  }
  requestDelete(book.value);
}

watch(id, () => {
  dismissActionError();
  void fetchDetail();
  if (isMobileRuntime()) {
    void refreshOfflineDownload();
  }
}, { immediate: true });
</script>

<style scoped>
.detail-shell {
  background: linear-gradient(145deg, #fbfaf7 0%, #f6f3ed 100%);
  min-height: calc(100vh - 60px);
  padding: 32px 28px 48px;
  width: 100%;
  font-family: 'Noto Sans TC Variable', 'Noto Sans TC', 'Avenir Next', sans-serif;
}

.detail-container {
  margin: 0 auto;
  max-width: 1120px;
  width: 100%;
}

.detail-panel {
  align-items: start;
  display: grid;
  gap: clamp(28px, 4vw, 48px);
  grid-template-columns: minmax(210px, 238px) minmax(0, 1fr);
  width: 100%;
}

.detail-cover-col {
  grid-area: auto;
  min-width: 0;
  position: sticky;
  top: 92px;
}

.detail-error {
  display: grid;
  gap: 10px;
}

.detail-error p {
  margin: 0;
}

.detail-error .button {
  justify-self: start;
}

.detail-notice {
  margin-bottom: 18px;
}

.reading-card {
  background: rgba(255, 255, 255, 0.76);
  border: 1px solid #e1ddd4;
  border-radius: 16px;
  box-shadow: 0 10px 30px rgba(61, 50, 35, 0.055);
  display: grid;
  gap: 12px;
  padding: 17px 18px 18px;
}

.progress-copy {
  align-items: end;
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.progress-eyebrow,
.progress-status {
  margin: 0;
}

.progress-eyebrow {
  color: #788391;
  font-size: 11px;
  font-weight: 750;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.progress-status {
  color: #283544;
  font-size: 16px;
  font-weight: 720;
  margin-top: 2px;
}

.progress-percent {
  color: #556273;
  font-size: 18px;
}

.reading-card :deep(.progress-bar) {
  --surface-muted: #e8e4dc;
  height: 7px;
}

.primary-actions {
  align-items: stretch;
  display: flex;
  flex-wrap: wrap;
  gap: 9px;
  margin-top: 2px;
}

.primary-actions .button {
  align-items: center;
  display: inline-flex;
  font-size: 14px;
  font-weight: 680;
  justify-content: center;
  min-height: 44px;
  padding: 10px 15px;
}

.read-button {
  min-width: 172px;
}

.more-trigger {
  gap: 8px;
}

.detail-actions-menu {
  min-width: 210px;
}

.detail-actions-menu .reka-menu-item {
  min-height: 44px;
}

.detail-menu-separator {
  border-top: 1px solid var(--border);
  margin: 5px 4px;
}

.detail-menu-danger {
  color: var(--danger, #cf3d3d);
}

.offline-download-error {
  color: #991b1b;
  font-size: 13px;
  margin: 0;
}

@media (max-width: 768px) {
  .detail-shell {
    min-height: calc(100vh - 54px);
    width: auto;
    margin: -16px calc(-24px - env(safe-area-inset-right, 0px))
      calc(-16px - env(safe-area-inset-bottom, 0px))
      calc(-24px - env(safe-area-inset-left, 0px));
    padding: 38px calc(40px + env(safe-area-inset-right, 0px))
      calc(52px + env(safe-area-inset-bottom, 0px))
      calc(40px + env(safe-area-inset-left, 0px));
  }

  .detail-panel {
    display: grid;
    gap: 18px 16px;
    grid-template-areas:
      'cover summary'
      'reading reading'
      'details details';
    grid-template-columns: 112px minmax(0, 1fr);
  }

  .detail-cover-col {
    grid-area: cover;
    position: static;
  }

  .reading-card {
    padding: 14px;
  }

  .primary-actions {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  .primary-actions .button {
    width: 100%;
  }

  .read-button {
    grid-column: 1 / -1;
    min-width: 0;
  }
}

@media (max-width: 360px) {
  .detail-panel {
    grid-template-areas:
      'summary'
      'cover'
      'reading'
      'details';
    grid-template-columns: minmax(0, 1fr);
  }

  .detail-cover-col {
    justify-self: center;
    width: 144px;
  }

  .primary-actions {
    grid-template-columns: 1fr;
  }

  .read-button {
    grid-column: auto;
  }
}
</style>
