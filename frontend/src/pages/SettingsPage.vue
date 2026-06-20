<template>
  <section class="settings-page">
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
        <div class="shelf-add-dir-row">
          <input
            v-model="newShelfDirectory"
            class="shelf-add-input shelf-add-dir-input"
            type="text"
            :placeholder="t('settings.shelves.addShelfDirectoryPlaceholder')"
            :disabled="addingShelf"
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
        <input
          v-model="newShelfScanInterval"
          class="shelf-add-input"
          type="text"
          :placeholder="t('settings.shelves.addShelfScanIntervalPlaceholder')"
          :disabled="addingShelf"
        />
        <p class="shelf-add-help">{{ t('settings.shelves.addShelfScanIntervalHelp') }}</p>
        <p v-if="addShelfError" class="settings-message settings-message-error" role="alert">
          {{ addShelfError }}
        </p>
      </form>
    </ConfirmModal>
    <header class="settings-header">
      <div>
        <h2>{{ t('settings.title') }}</h2>
        <p>{{ t('settings.description') }}</p>
      </div>
      <button class="button" type="button" :disabled="loading || saving" @click="loadSettings">
        {{ t('common.retry') }}
      </button>
    </header>

    <p v-if="error" class="settings-message settings-message-error" role="alert">
      {{ error }}
    </p>

    <section class="panel settings-group">
      <h3>{{ t('settings.cover.title') }}</h3>
      <label class="setting-item">
        <div>
          <div class="setting-label">{{ t('settings.coverToJpg.label') }}</div>
          <p class="setting-description">{{ t('settings.coverToJpg.description') }}</p>
        </div>
        <input
          class="setting-checkbox"
          type="checkbox"
          :checked="coverToJpg"
          :disabled="loading || saving"
          @change="onCoverToJpgChange"
        />
      </label>
    </section>

    <section class="panel settings-group">
      <h3>{{ t('settings.readHistory.title') }}</h3>
      <label class="setting-item">
        <div>
          <div class="setting-label">{{ t('settings.readHistoryLimit.label') }}</div>
          <p class="setting-description">{{ t('settings.readHistoryLimit.description') }}</p>
        </div>
        <input
          class="setting-number"
          type="number"
          inputmode="numeric"
          min="0"
          step="1"
          :value="readHistoryLimit"
          :disabled="loading || saving"
          @change="onReadHistoryLimitChange"
        />
      </label>
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
              <th>ID</th>
              <th v-if="isDesktopEnv"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="shelf in shelves" :key="shelf.id">
              <td>{{ shelf.name }}</td>
              <td class="shelf-id-cell">{{ shelf.id }}</td>
              <td v-if="isDesktopEnv" class="shelf-action-cell">
                <template v-if="confirmingShelfID === shelf.id">
                  <span class="shelf-confirm-label">{{ t('settings.shelves.removeConfirmInline') }}</span>
                  <button
                    type="button"
                    class="shelf-confirm-btn"
                    :disabled="removingShelfIDs.has(shelf.id)"
                    @click="onRemoveShelf(shelf)"
                  >
                    {{ removingShelfIDs.has(shelf.id) ? t('settings.shelves.removing') : t('settings.shelves.removeConfirmYes') }}
                  </button>
                  <button
                    type="button"
                    class="shelf-cancel-btn"
                    :disabled="removingShelfIDs.has(shelf.id)"
                    @click="confirmingShelfID = ''"
                  >
                    {{ t('common.cancel') }}
                  </button>
                </template>
                <button
                  v-else
                  type="button"
                  class="shelf-remove-btn"
                  @click="confirmingShelfID = shelf.id"
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
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import ConfirmModal from '../components/ConfirmModal.vue';
import {
  getCoverToJpgSetting,
  getReadHistoryLimitSetting,
  setCoverToJpgSetting,
  setReadHistoryLimitSetting
} from '../api/settings';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useShelvesStore } from '../composables/useShelvesStore';
import { useI18n } from '../i18n';
import { getBookshelfProvider, isWailsRuntime } from '../providers';

const { t } = useI18n();
const loading = ref(false);
const saving = ref(false);
const error = ref('');
const coverToJpg = ref(false);
const readHistoryLimit = ref(0);

const isDesktopEnv = computed(() => isWailsRuntime());
const { shelves, loading: shelvesLoading, error: shelvesError, fetchShelves } = useShelvesStore();
const shelfOpError = ref('');
const removingShelfIDs = ref<Set<string>>(new Set());
const confirmingShelfID = ref('');
const showAddShelfModal = ref(false);
const newShelfName = ref('');
const newShelfDirectory = ref('');
const newShelfScanInterval = ref('');
const addingShelf = ref(false);
const addShelfError = ref('');
const canSubmitAddShelf = computed(
  () => newShelfName.value.trim().length > 0 && newShelfDirectory.value.trim().length > 0
);

useDocumentTitle(() => [t('settings.title'), t('app.name')]);

async function loadSettings(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    const [nextCoverToJpg, nextReadHistoryLimit] = await Promise.all([
      getCoverToJpgSetting(),
      getReadHistoryLimitSetting()
    ]);
    coverToJpg.value = nextCoverToJpg;
    readHistoryLimit.value = nextReadHistoryLimit;
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('settings.loadFailed');
  } finally {
    loading.value = false;
  }
}

function parseReadHistoryLimit(value: string): number | null {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}

async function onCoverToJpgChange(event: Event): Promise<void> {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }

  const nextValue = target.checked;
  const prevValue = coverToJpg.value;
  coverToJpg.value = nextValue;
  saving.value = true;
  error.value = '';

  try {
    await setCoverToJpgSetting(nextValue);
  } catch (err) {
    coverToJpg.value = prevValue;
    target.checked = prevValue;
    error.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

async function onReadHistoryLimitChange(event: Event): Promise<void> {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }

  const nextValue = parseReadHistoryLimit(target.value);
  const prevValue = readHistoryLimit.value;
  if (nextValue === null) {
    target.value = String(prevValue);
    error.value = t('settings.readHistoryLimit.invalid');
    return;
  }

  readHistoryLimit.value = nextValue;
  saving.value = true;
  error.value = '';

  try {
    await setReadHistoryLimitSetting(nextValue);
  } catch (err) {
    readHistoryLimit.value = prevValue;
    target.value = String(prevValue);
    error.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

async function onRemoveShelf(shelf: { id: string; name: string }): Promise<void> {
  const provider = getBookshelfProvider();
  if (!provider.removeDesktopShelf) {
    return;
  }

  removingShelfIDs.value = new Set([...removingShelfIDs.value, shelf.id]);

  shelfOpError.value = '';
  try {
    await provider.removeDesktopShelf(shelf.id);
    await fetchShelves();
  } catch (err) {
    shelfOpError.value = err instanceof Error ? err.message : t('settings.shelves.removeFailed');
  } finally {
    const next = new Set(removingShelfIDs.value);
    next.delete(shelf.id);
    removingShelfIDs.value = next;
    confirmingShelfID.value = '';
  }
}

function resetAddShelfForm(): void {
  newShelfName.value = '';
  newShelfDirectory.value = '';
  newShelfScanInterval.value = '';
  addShelfError.value = '';
}

function openAddShelfModal(): void {
  resetAddShelfForm();
  showAddShelfModal.value = true;
}

function closeAddShelfModal(): void {
  if (addingShelf.value) {
    return;
  }

  showAddShelfModal.value = false;
  resetAddShelfForm();
}

async function onBrowseShelfDirectory(): Promise<void> {
  const provider = getBookshelfProvider();
  if (!provider.openDesktopShelfDirectory) {
    return;
  }
  const dir = await provider.openDesktopShelfDirectory();
  if (dir) {
    newShelfDirectory.value = dir;
  }
}

async function onSubmitAddShelf(): Promise<void> {
  const name = newShelfName.value.trim();
  const dir = newShelfDirectory.value.trim();
  const scanInterval = newShelfScanInterval.value.trim();
  if (!name || !dir) {
    return;
  }

  addingShelf.value = true;
  addShelfError.value = '';

  try {
    const provider = getBookshelfProvider();
    await provider.addDesktopShelf!(name, dir, scanInterval);
    await fetchShelves();
    showAddShelfModal.value = false;
    resetAddShelfForm();
  } catch (err) {
    addShelfError.value = err instanceof Error ? err.message : t('settings.shelves.addShelfFailed');
  } finally {
    addingShelf.value = false;
  }
}

onMounted(() => {
  void loadSettings();
  void fetchShelves();
});
</script>

<style scoped>
.settings-page {
  display: grid;
  gap: 16px;
  max-width: 760px;
}

.settings-header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.settings-header h2 {
  margin: 0;
}

.settings-header p {
  color: #475569;
  margin: 6px 0 0;
}

.settings-message {
  margin: 0;
}

.settings-message-error {
  color: #b91c1c;
}

.settings-group {
  display: grid;
  gap: 12px;
  padding: 16px;
}

.settings-group h3 {
  margin: 0;
}

.setting-item {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.setting-label {
  font-weight: 600;
}

.setting-description {
  color: #475569;
  font-size: 13px;
  margin: 4px 0 0;
}

.setting-checkbox {
  cursor: pointer;
  height: 18px;
  width: 18px;
}

.setting-checkbox:disabled {
  cursor: not-allowed;
}

.setting-number {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  font: inherit;
  max-width: 120px;
  padding: 8px 10px;
}

.setting-number:disabled {
  cursor: not-allowed;
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

.shelf-confirm-label {
  color: #b91c1c;
  font-size: 12px;
  font-weight: 600;
  margin-right: 6px;
}

.shelf-confirm-btn {
  background: #b91c1c;
  border: 1px solid #991b1b;
  border-radius: 6px;
  color: #ffffff;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  margin-right: 4px;
  padding: 3px 10px;
}

.shelf-confirm-btn:hover:not(:disabled) {
  background: #991b1b;
}

.shelf-confirm-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-cancel-btn {
  background: transparent;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 3px 10px;
}

.shelf-cancel-btn:hover:not(:disabled) {
  background: #f1f5f9;
}

.shelf-cancel-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
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

.shelf-add-fields {
  display: flex;
  flex-direction: column;
  gap: 6px;
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

.shelf-add-actions {
  display: flex;
  justify-content: flex-end;
}

.create-layer-submit {
  background: #2563eb;
  border: 1px solid #1d4ed8;
  border-radius: 6px;
  color: #ffffff;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 14px;
}

.create-layer-submit:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
