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
      <input
        v-model="modifyShelfName"
        class="shelf-add-input"
        type="text"
        :placeholder="t('settings.shelves.modifyShelfNamePlaceholder')"
        :disabled="modifyingShelf"
        autofocus
      />
      <ScanIntervalField v-model="modifyShelfScanInterval" :disabled="modifyingShelf" />
      <ShelfReadOnlyField v-model="modifyShelfReadOnly" :disabled="modifyingShelf" />
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
    :confirm-text="t('settings.shelves.addShelfSubmit')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('settings.shelves.addShelfAdding')"
    :busy="addingShelf"
    :confirm-disabled="!canSubmitAddShelf"
    :close-label="t('settings.shelves.addShelfCloseLabel')"
    @cancel="closeAddShelfModal"
    @confirm="onSubmitAddShelf"
  >
    <form class="shelf-add-form" @submit.prevent="onSubmitAddShelf">
      <input
        v-model="newShelfName"
        class="shelf-add-input"
        type="text"
        :placeholder="t('settings.shelves.addShelfNamePlaceholder')"
        :disabled="addingShelf"
        autofocus
      />
      <!-- The two ways to place a shelf answer the question the single path box
           left the user to guess: whether PlainShelf is about to create a folder
           on their disk or open one they already have. Only the second branch
           can be read-only, so the toggle lives inside it. -->
      <RadioGroupRoot
        v-model="newShelfLocationMode"
        class="shelf-location-modes"
        :disabled="addingShelf"
        :aria-label="t('settings.shelves.addShelfLocationLabel')"
      >
        <RadioGroupItem class="shelf-location-mode" value="new" data-testid="shelf-location-new">
          <RadioGroupIndicator class="shelf-location-check" aria-hidden="true">●</RadioGroupIndicator>
          <span class="shelf-location-copy">
            <strong>{{ t('settings.shelves.addShelfLocationNew') }}</strong>
            <span class="shelf-add-help">{{ t('settings.shelves.addShelfLocationNewHelp') }}</span>
          </span>
        </RadioGroupItem>
        <RadioGroupItem
          class="shelf-location-mode"
          value="existing"
          data-testid="shelf-location-existing"
        >
          <RadioGroupIndicator class="shelf-location-check" aria-hidden="true">●</RadioGroupIndicator>
          <span class="shelf-location-copy">
            <strong>{{ t('settings.shelves.addShelfLocationExisting') }}</strong>
            <span class="shelf-add-help">{{
              t('settings.shelves.addShelfLocationExistingHelp')
            }}</span>
          </span>
        </RadioGroupItem>
      </RadioGroupRoot>

      <template v-if="newShelfLocationMode === 'existing'">
        <div class="shelf-add-dir-row">
          <input
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

      <!-- Previews what submitting would create: on the default branch the
           folder that is actually sent as lib_root, and either way the id it
           would be frozen with. -->
      <p v-if="newShelfIDPreview" class="shelf-add-help shelf-id-preview">
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

  <section v-if="isMobileEnv" class="panel settings-group">
    <h3>{{ t('settings.mobileConnect.title') }}</h3>
    <p class="setting-description">{{ t('settings.mobileConnect.description') }}</p>
    <RouterLink to="/connect" class="button mobile-connect-link">
      {{ t('settings.mobileConnect.open') }}
    </RouterLink>
  </section>

  <section v-if="isMobileEnv" class="panel settings-group">
    <h3>{{ t('settings.downloads.title') }}</h3>
    <p class="setting-description">{{ t('settings.downloads.description') }}</p>
    <RouterLink to="/downloads" class="button mobile-connect-link">
      {{ t('settings.downloads.open') }}
    </RouterLink>
  </section>

  <section class="panel settings-group">
    <h3>{{ t('settings.shelves.title') }}</h3>

    <p v-if="shelvesError || shelfOpError" class="settings-message settings-message-error" role="alert">
      {{ shelfOpError || shelvesError }}
    </p>

    <p v-if="!isDesktopEnv" class="setting-description">
      {{ t('settings.shelves.serverManaged') }}
    </p>

    <div v-if="shelvesLoading" class="setting-description">{{ t('layout.shelf.loading') }}</div>
    <template v-else>
      <p v-if="shelves.length === 0" class="setting-description">
        {{ t('settings.shelves.empty') }}
      </p>
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
            <td>{{ shelf.name }}</td>
            <td class="shelf-id-cell">{{ shelf.id }}</td>
            <td v-if="isDesktopEnv" class="shelf-action-cell">
              <button
                type="button"
                class="shelf-modify-btn"
                :disabled="removingShelfIDs.has(shelf.id)"
                @click="openShelfFolder(shelf.id)"
              >
                {{ t('settings.shelves.openFolder') }}
              </button>
              <button
                type="button"
                class="shelf-modify-btn"
                :disabled="removingShelfIDs.has(shelf.id)"
                @click="requestModifyShelf(shelf)"
              >
                {{ t('settings.shelves.modify') }}
              </button>
              <button
                type="button"
                class="shelf-remove-btn"
                :disabled="removingShelfIDs.has(shelf.id)"
                @click="requestRemoveShelf(shelf)"
              >
                {{ t('settings.shelves.remove') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>

    <div v-if="isDesktopEnv" class="shelf-add-row">
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

  <!-- The export is written by whatever serves these shelves, so the panel
       belongs to the server and desktop shells and not to the mobile one,
       which is the client that reads the file. serverSettingsEditable is that
       platform question already: it is false on the mobile shell. It is
       deliberately not the server's own read_only mode, which leaves these
       panels rendered and disabled rather than dropping them. -->
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
        :disabled="bookCacheExporting || shelves.length === 0"
        @click="onExportBookCache"
      >
        {{ bookCacheExporting ? t('settings.bookCache.exporting') : t('settings.bookCache.export') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RadioGroupIndicator, RadioGroupItem, RadioGroupRoot } from 'reka-ui';
import ConfirmModal from '@/components/ConfirmModal.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import ScanIntervalField from '@/features/settings/components/ScanIntervalField.vue';
import BookCheckIntervalField from '@/features/settings/components/BookCheckIntervalField.vue';
import ShelfReadOnlyField from '@/features/settings/components/ShelfReadOnlyField.vue';
import { exportShelfBookCache } from '@/api/shelves';
import { useI18n } from '@/i18n';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { isMobileRuntime, isWailsRuntime } from '@/providers';
import { useShelfManagement } from '@/features/settings/composables/useShelfManagement';

const { t } = useI18n();
const { serverSettingsEditable } = useWriteAccess();
const isDesktopEnv = computed(() => isWailsRuntime());
const isMobileEnv = computed(() => isMobileRuntime());

const bookCacheExporting = ref(false);
const bookCacheExported = ref(false);
const bookCacheError = ref('');

const {
  shelves,
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
  showAddShelfModal,
  newShelfName,
  newShelfLocationMode,
  newShelfDirectory,
  newShelfDirectoryError,
  newShelfReadOnly,
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
  modifyingShelf,
  modifyShelfError,
  canSubmitModifyShelf,
  requestModifyShelf,
  closeModifyShelfModal,
  onSubmitModifyShelf
} = useShelfManagement();

/**
 * Rewrites every listed shelf's exported book cache.
 *
 * Sequential rather than concurrent: each call makes the server walk a shelf,
 * and on a network shelf several walks at once is exactly the load this cache
 * exists to avoid.
 */
async function onExportBookCache(): Promise<void> {
  bookCacheError.value = '';
  bookCacheExported.value = false;
  bookCacheExporting.value = true;

  try {
    // A read-only shelf has no exported cache to rewrite — the server does not
    // write one for it — and asking anyway answers 409, which would abort the
    // loop and leave every shelf after it untouched.
    for (const shelf of shelves.value.filter((entry) => !entry.readOnly)) {
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
.mobile-connect-link {
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

.shelf-location-modes {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.shelf-location-mode {
  align-items: flex-start;
  background: transparent;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  gap: 8px;
  padding: 8px 10px;
  text-align: left;
  width: 100%;
}

.shelf-location-mode[data-state='checked'] {
  border-color: #64748b;
}

.shelf-location-mode:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-location-check {
  color: #334155;
  font-size: 12px;
  line-height: 18px;
  min-width: 10px;
}

.shelf-location-copy {
  display: flex;
  flex-direction: column;
  font-size: 13px;
  gap: 2px;
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
