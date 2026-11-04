<template>
  <DeleteModal
    :open="pendingRemoveShelf !== null"
    :title="t('settings.shelves.removeShelfTitle')"
    :item-name="pendingRemoveShelf?.name ?? ''"
    :description="pendingRemoveShelf ? t('settings.shelves.removeConfirmDescription') : ''"
    :confirm-text="t('settings.shelves.removeConfirmYes')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('settings.shelves.removing')"
    :busy="pendingRemoveShelf ? removingShelfIDs.has(pendingRemoveShelf.id) : false"
    :error="shelfRemoveModalError"
    @cancel="cancelRemoveShelf"
    @confirm="confirmRemoveShelf"
  />

  <ConfirmModal
    :open="showModifyShelfModal"
    :title="t('settings.shelves.modifyShelfTitle')"
    :confirm-text="t('settings.shelves.modifyShelfSubmit')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('settings.shelves.modifyShelfSaving')"
    :busy="modifyingShelf"
    :confirm-disabled="!canSubmitModifyShelf"
    :close-label="t('settings.shelves.modifyShelfCloseLabel')"
    :initial-focus="`#${modifyShelfNameId}`"
    @cancel="closeModifyShelfModal"
    @confirm="onSubmitModifyShelf"
  >
    <form class="shelf-add-form" @submit.prevent="onSubmitModifyShelf">
      <div class="shelf-modify-readonly-row">
        <span class="shelf-modify-label">{{ t('settings.shelves.modifyShelfIDLabel') }}</span>
        <span class="shelf-modify-value shelf-id-cell">{{ pendingModifyShelf?.id }}</span>
      </div>
      <div class="shelf-modify-readonly-row">
        <span class="shelf-modify-label">{{ t('settings.shelves.modifyShelfPathLabel') }}</span>
        <span class="shelf-modify-value">{{ modifyShelfPath }}</span>
      </div>
      <div class="shelf-field">
        <label class="shelf-field-label" :for="modifyShelfNameId">
          {{ t('settings.shelves.modifyShelfNameLabel') }}
        </label>
        <input
          :id="modifyShelfNameId"
          v-model="modifyShelfName"
          class="shelf-add-input"
          type="text"
          :placeholder="t('settings.shelves.modifyShelfNamePlaceholder')"
          :disabled="modifyingShelf"
        />
      </div>
      <ScanIntervalField v-model="modifyShelfScanInterval" :disabled="modifyingShelf" />
      <ShelfReadOnlyField
        v-if="!modifyShelfIsVirtual"
        v-model="modifyShelfReadOnly"
        :disabled="modifyingShelf || modifyShelfIsVirtual"
      />
      <details class="shelf-advanced">
        <summary class="shelf-advanced-summary">
          {{ t('settings.shelves.advancedSettings') }}
        </summary>
        <div class="shelf-advanced-body">
          <BookCheckIntervalField
            v-model="modifyShelfBookCheckInterval"
            :disabled="modifyingShelf"
          />
        </div>
      </details>
      <p v-if="modifyShelfError" class="settings-message settings-message-error" role="alert">
        {{ modifyShelfError }}
      </p>
    </form>
  </ConfirmModal>

  <ConfirmModal
    :open="showAddShelfModal"
    :title="t('settings.shelves.addShelfTitle')"
    :confirm-text="newShelfLocationMode === 'qr' ? t('settings.shelves.qrConfirm') : t('settings.shelves.addShelfSubmit')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('settings.shelves.addShelfAdding')"
    :busy="addingShelf"
    :confirm-disabled="!canSubmitAddShelf"
    :close-label="t('settings.shelves.addShelfCloseLabel')"
    :initial-focus="`#${addShelfNameId}`"
    @cancel="closeAddShelfModal"
    @confirm="onSubmitAddShelf"
  >
    <form class="shelf-add-form" @submit.prevent="onSubmitAddShelf">
      <div v-if="newShelfLocationMode !== 'qr'" class="shelf-field">
        <label class="shelf-field-label" :for="addShelfNameId">
          {{ t('settings.shelves.addShelfNameLabel') }}
        </label>
        <input
          :id="addShelfNameId"
          v-model="newShelfName"
          class="shelf-add-input"
          type="text"
          data-testid="shelf-name-input"
          :placeholder="t('settings.shelves.addShelfNamePlaceholder')"
          :disabled="addingShelf"
        />
      </div>
      <ShelfSourceModes
        v-model="newShelfLocationMode"
        :options="locationModeOptions"
        :disabled="addingShelf"
        :label="t('settings.shelves.addShelfLocationLabel')"
      />
      <template v-if="newShelfLocationMode === 'qr'">
        <label class="shelf-field-label" :for="addShelfQRId">{{ t('settings.shelves.qrPaste') }}</label>
        <textarea
          :id="addShelfQRId"
          v-model="qrText"
          class="shelf-add-input"
          :disabled="addingShelf"
          spellcheck="false"
          autocomplete="off"
          data-testid="shelf-qr-json"
        />
        <button
          type="button"
          class="shelf-browse-btn"
          :disabled="addingShelf || !qrText.trim()"
          @click="previewQR"
        >
          {{ t('settings.shelves.qrPreview') }}
        </button>
        <p class="shelf-add-help">{{ t('settings.shelves.qrHelp') }}</p>
        <ul v-if="qrShelves.length" data-testid="shelf-qr-preview">
          <li v-for="shelf in qrShelves" :key="shelf.id">{{ shelf.name }} — {{ shelf.url }}</li>
        </ul>
        <ul v-if="qrResults.length" role="status" data-testid="shelf-qr-results">
          <li v-for="(result, index) in qrResults" :key="index">{{ result.name }} — {{ qrStatus(result.status) }}</li>
        </ul>
      </template>

      <RemoteSourceFields
        v-if="newShelfLocationMode === 'remote'"
        v-model:kind="newShelfSourceKind"
        v-model:url="newShelfSourceURL"
        v-model:username="newShelfSourceUsername"
        v-model:secret="newShelfSourceSecret"
        :disabled="addingShelf"
      />

      <template v-if="newShelfLocationMode === 'existing'">
        <div class="shelf-field">
          <label class="shelf-field-label" :for="addShelfDirectoryId">
            {{ t('settings.shelves.addShelfDirectoryLabel') }}
          </label>
          <div class="shelf-add-dir-row">
            <input
              :id="addShelfDirectoryId"
              v-model="newShelfDirectory"
              class="shelf-add-input shelf-add-dir-input"
              type="text"
              :placeholder="t('settings.shelves.addShelfDirectoryPlaceholder')"
              :disabled="addingShelf"
              data-testid="shelf-directory-input"
            />
            <button
              type="button"
              class="shelf-browse-btn"
              :disabled="addingShelf"
              @click="onBrowseShelfDirectory"
            >
              {{ t('settings.shelves.addShelfBrowse') }}
            </button>
          </div>
        </div>
        <p
          v-if="newShelfDirectoryError"
          class="settings-message settings-message-error"
          role="alert"
          data-testid="shelf-directory-error"
        >
          {{ newShelfDirectoryError }}
        </p>
        <ShelfReadOnlyField v-model="newShelfReadOnly" :disabled="addingShelf" />
      </template>

      <p v-if="newShelfLocationMode !== 'qr' && newShelfIDPreview" class="shelf-add-help shelf-id-preview">
        <span
          v-if="newShelfLocationMode === 'new' && newShelfEffectiveDirectory"
          class="shelf-preview-line"
          data-testid="shelf-default-path"
        >
          {{ t('settings.shelves.addShelfDefaultPath') }}
          <span class="shelf-preview-path">{{ newShelfEffectiveDirectory }}</span>
        </span>
        <span class="shelf-preview-line">
          {{ t('settings.shelves.addShelfIDPreview') }}
          <span class="shelf-id-cell">{{ newShelfIDPreview }}</span>
        </span>
      </p>
      <p v-if="addShelfError" class="settings-message settings-message-error" role="alert">
        {{ addShelfError }}
      </p>
    </form>
  </ConfirmModal>

  <MobileShelvesPanel v-if="isMobileEnv" />

  <section v-if="isMobileEnv" class="panel settings-group">
    <h3>{{ t('settings.downloads.title') }}</h3>
    <p class="setting-description">{{ t('settings.downloads.description') }}</p>
    <RouterLink to="/downloads" class="button mobile-downloads-link">
      {{ t('settings.downloads.open') }}
    </RouterLink>
  </section>

  <section v-if="!isMobileEnv" class="panel settings-group">
    <h3>{{ t('settings.shelves.title') }}</h3>

    <p v-if="shelvesError || shelfOpError" class="settings-message settings-message-error" role="alert">
      {{ shelfOpError || shelvesError }}
    </p>

    <p v-if="!isDesktopEnv" class="setting-description">
      {{ t('settings.shelves.serverManaged') }}
    </p>

    <div v-if="shelvesLoading" class="setting-description">{{ t('layout.shelf.loading') }}</div>
    <template v-else>
      <div v-if="shelves.length === 0" class="shelf-empty">
        <p class="setting-description">{{ t('settings.shelves.empty') }}</p>
        <button
          v-if="isDesktopEnv"
          type="button"
          class="button primary shelf-empty-add"
          :disabled="addingShelf"
          @click="openAddShelfModal"
        >
          {{ t('settings.shelves.addShelf') }}
        </button>
      </div>
      <table v-else class="shelves-table">
        <thead>
          <tr>
            <th>{{ t('settings.shelves.name') }}</th>
            <th>{{ t('settings.shelves.idColumn') }}</th>
            <th v-if="isDesktopEnv"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="shelf in shelves" :key="shelf.id">
            <td>
              {{ shelf.name }}
              <template v-if="shelf.source">
                <p class="setting-description">{{ shelf.source }} · {{ shelf.url }}</p>
                <p
                  v-if="shelf.open_error"
                  class="settings-message settings-message-error"
                  role="alert"
                >
                  {{ t('settings.shelves.virtualOpenFailed') }}
                </p>
              </template>
            </td>
            <td class="shelf-id-cell">{{ shelf.id }}</td>
            <td v-if="isDesktopEnv" class="shelf-action-cell">
              <button
                type="button"
                class="shelf-modify-btn"
                :disabled="removingShelfIDs.has(shelf.id) || retryingShelfIDs.has(shelf.id)"
                v-if="!shelf.source"
                @click="openShelfFolder(shelf.id)"
              >
                {{ t('settings.shelves.openFolder') }}
              </button>
              <button
                v-if="shelf.open_error"
                type="button"
                class="shelf-modify-btn"
                :disabled="removingShelfIDs.has(shelf.id) || retryingShelfIDs.has(shelf.id)"
                @click="retryShelf(shelf.id)"
              >
                {{ t('common.retry') }}
              </button>
              <button
                v-else
                type="button"
                class="shelf-modify-btn"
                :disabled="removingShelfIDs.has(shelf.id) || retryingShelfIDs.has(shelf.id)"
                @click="requestModifyShelf(shelf)"
              >
                {{ t('settings.shelves.modify') }}
              </button>
              <button
                type="button"
                class="shelf-remove-btn"
                :disabled="removingShelfIDs.has(shelf.id) || retryingShelfIDs.has(shelf.id)"
                @click="requestRemoveShelf(shelf)"
              >
                {{ t('settings.shelves.remove') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>

    <div v-if="isDesktopEnv && shelves.length > 0" class="shelf-add-row">
      <button
        type="button"
        class="shelf-add-toggle"
        :disabled="addingShelf"
        @click="openAddShelfModal"
      >
        {{ t('settings.shelves.addShelf') }}
      </button>
    </div>
  </section>

  <section v-if="serverSettingsEditable" class="panel settings-group">
    <h3>{{ t('settings.bookCache.title') }}</h3>
    <p class="setting-description">{{ t('settings.bookCache.description') }}</p>

    <p v-if="bookCacheError" class="settings-message settings-message-error" role="alert">
      {{ bookCacheError }}
    </p>
    <p v-else-if="bookCacheExported" class="settings-message" role="status">
      {{ t('settings.bookCache.success') }}
    </p>

    <div class="shelf-add-row">
      <button
        type="button"
        class="shelf-add-toggle"
        :disabled="bookCacheExporting || exportableShelves.length === 0"
        @click="onExportBookCache"
      >
        {{ bookCacheExporting ? t('settings.bookCache.exporting') : t('settings.bookCache.export') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, useId } from 'vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import RemoteSourceFields from '@/components/shelfSource/RemoteSourceFields.vue';
import ShelfSourceModes from '@/components/shelfSource/ShelfSourceModes.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import ScanIntervalField from '@/features/settings/components/ScanIntervalField.vue';
import BookCheckIntervalField from '@/features/settings/components/BookCheckIntervalField.vue';
import ShelfReadOnlyField from '@/features/settings/components/ShelfReadOnlyField.vue';
import { exportShelfBookCache } from '@/api/shelves';
import { useI18n } from '@/i18n';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { isMobileRuntime } from '@/providers';
import { useShelfManagement } from '@/features/settings/composables/useShelfManagement';

const MobileShelvesPanel = defineAsyncComponent(
  () => import('@/features/mobile/components/MobileShelvesPanel.vue')
);

const { t } = useI18n();
const { serverSettingsEditable } = useWriteAccess();
const isDesktopEnv = computed(() => Boolean(getPlatformCapabilities().shelves));
const isMobileEnv = computed(() => isMobileRuntime());

const addShelfQRId = `shelf-add-qr-${useId()}`;
const addShelfNameId = `shelf-add-name-${useId()}`;
const addShelfDirectoryId = `shelf-add-directory-${useId()}`;
const modifyShelfNameId = `shelf-modify-name-${useId()}`;

const bookCacheExporting = ref(false);
const bookCacheExported = ref(false);
const bookCacheError = ref('');

const {
  shelves,
  retryShelf,
  retryingShelfIDs,
  shelvesLoading,
  shelvesError,
  ensureShelvesLoaded,
  shelfOpError,
  removingShelfIDs,
  pendingRemoveShelf,
  shelfRemoveModalError,
  requestRemoveShelf,
  cancelRemoveShelf,
  confirmRemoveShelf,
  qrText,
  qrShelves,
  qrResults,
  previewQR,
  qrStatus,
  showAddShelfModal,
  newShelfName,
  newShelfLocationMode,
  newShelfDirectory,
  newShelfDirectoryError,
  newShelfReadOnly,
  newShelfSourceKind,
  newShelfSourceURL,
  newShelfSourceUsername,
  newShelfSourceSecret,
  newShelfIDPreview,
  newShelfEffectiveDirectory,
  addingShelf,
  addShelfError,
  canSubmitAddShelf,
  openAddShelfModal,
  closeAddShelfModal,
  onBrowseShelfDirectory,
  onSubmitAddShelf,
  openShelfFolder,
  pendingModifyShelf,
  showModifyShelfModal,
  modifyShelfName,
  modifyShelfScanInterval,
  modifyShelfBookCheckInterval,
  modifyShelfReadOnly,
  modifyShelfPath,
  modifyShelfIsVirtual,
  modifyingShelf,
  modifyShelfError,
  canSubmitModifyShelf,
  requestModifyShelf,
  closeModifyShelfModal,
  onSubmitModifyShelf
} = useShelfManagement();

const locationModeOptions = computed(() => [
  {
    value: 'new' as const,
    label: t('settings.shelves.addShelfLocationNew'),
    help: t('settings.shelves.addShelfLocationNewHelp'),
    testId: 'shelf-location-new'
  },
  {
    value: 'existing' as const,
    label: t('settings.shelves.addShelfLocationExisting'),
    help: t('settings.shelves.addShelfLocationExistingHelp'),
    testId: 'shelf-location-existing'
  },
  {
    value: 'remote' as const,
    label: t('shelfSource.modeRemote'),
    help: t('shelfSource.modeRemoteHelp'),
    testId: 'shelf-location-remote'
  },
  ...(getPlatformCapabilities().shelves?.previewLANShareQR
    ? [{
        value: 'qr' as const,
        label: t('settings.shelves.qrTitle'),
        help: t('shelfSource.modeQRHelp'),
        testId: 'shelf-location-qr'
      }]
    : [])
]);

const exportableShelves = computed(() => shelves.value.filter((shelf) => !shelf.readOnly));

async function onExportBookCache(): Promise<void> {
  bookCacheError.value = '';
  bookCacheExported.value = false;
  bookCacheExporting.value = true;

  try {
    for (const shelf of exportableShelves.value) {
      await exportShelfBookCache(shelf.id);
    }
    bookCacheExported.value = true;
  } catch (err) {
    bookCacheError.value = err instanceof Error ? err.message : t('settings.bookCache.failed');
  } finally {
    bookCacheExporting.value = false;
  }
}

onMounted(() => {
  void ensureShelvesLoaded();
});
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.mobile-downloads-link {
  justify-self: start;
  text-decoration: none;
}

/* Collapsed by default so the two esoteric per-shelf knobs (only book_check_interval
   for now) stay out of the way of the common name/directory/scan-interval fields, and
   a first-time user does not trip over them. */
.shelf-advanced {
  border-top: 1px solid #e2e8f0;
  padding-top: 8px;
}

.shelf-advanced-summary {
  color: #64748b;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.03em;
  list-style: revert;
  user-select: none;
}

.shelf-advanced-body {
  padding-top: 10px;
}

.shelves-table {
  border-collapse: collapse;
  font-size: 13px;
  width: 100%;
}

.shelves-table th {
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  padding: 4px 8px 6px;
  text-align: left;
  text-transform: uppercase;
}

.shelves-table td {
  border-top: 1px solid #e2e8f0;
  padding: 8px;
  vertical-align: middle;
}

.shelf-id-cell {
  color: #64748b;
  font-family: monospace;
  font-size: 12px;
}

.shelf-action-cell {
  text-align: right;
  white-space: nowrap;
}

.shelf-modify-btn {
  background: transparent;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #475569;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  margin-right: 6px;
  padding: 3px 10px;
}

.shelf-modify-btn:hover:not(:disabled) {
  background: #f1f5f9;
}

.shelf-modify-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-remove-btn {
  background: transparent;
  border: 1px solid #fca5a5;
  border-radius: 6px;
  color: #b91c1c;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 3px 10px;
}

.shelf-remove-btn:hover:not(:disabled) {
  background: #fef2f2;
}

.shelf-remove-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-modify-readonly-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.shelf-modify-label {
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.shelf-modify-value {
  font-size: 13px;
  word-break: break-all;
}

.shelf-empty {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: space-between;
}

.shelf-empty-add {
  flex: none;
}

.shelf-add-row {
  display: flex;
}

.shelf-add-toggle {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 12px;
}

.shelf-add-toggle:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-add-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.shelf-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* Same small-caps field label the interval controls and the modify dialog's
   read-only rows already use. */
.shelf-field-label,
.shelf-add-form :deep(.remote-source-label) {
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.shelf-add-input {
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  padding: 7px 10px;
}

.shelf-add-input:disabled {
  cursor: not-allowed;
}

.shelf-add-help {
  color: #64748b;
  font-size: 12px;
  margin: -2px 0 0;
}

/* The preview stacks two labelled lines; the path can be long, so it wraps
   rather than widening the dialog. */
.shelf-preview-line {
  display: block;
}

.shelf-preview-path {
  overflow-wrap: anywhere;
}

.shelf-add-dir-row {
  display: flex;
  gap: 6px;
}

.shelf-add-dir-input {
  flex: 1;
  min-width: 0;
}

.shelf-browse-btn {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 6px 10px;
  white-space: nowrap;
}

.shelf-browse-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

</style>
