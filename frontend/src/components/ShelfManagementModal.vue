<template>
  <Teleport to="body">
    <div v-if="open" class="shelf-modal-overlay" role="presentation" @click="onBackdropClick">
      <section class="panel shelf-modal" role="dialog" aria-modal="true" :aria-labelledby="titleId" @click.stop>
        <header class="shelf-modal-header">
          <div>
            <h2 :id="titleId">{{ t('shelfModal.title') }}</h2>
            <p>{{ t('shelfModal.description') }}</p>
          </div>
          <button
            class="shelf-modal-close"
            type="button"
            :aria-label="t('shelfModal.closeLabel')"
            :disabled="operationInProgress"
            @click="requestClose"
          >
            ×
          </button>
        </header>

        <p v-if="errorMessage" class="shelf-modal-message shelf-modal-message-error" role="alert">
          {{ errorMessage }}
        </p>

        <section class="shelf-modal-section" :aria-label="t('shelfModal.listLabel')">
          <div v-if="loading" class="shelf-modal-muted">{{ t('layout.shelf.loading') }}</div>
          <p v-else-if="shelves.length === 0" class="shelf-modal-muted">{{ t('settings.shelves.empty') }}</p>
          <ul v-else class="shelf-modal-list">
            <li v-for="shelf in shelves" :key="shelf.id" class="shelf-modal-item">
              <div class="shelf-modal-item-copy">
                <strong>{{ shelf.name }}</strong>
                <span>{{ shelf.id }}</span>
              </div>

              <div v-if="confirmingShelfID === shelf.id" class="shelf-modal-confirm">
                <span>{{ t('settings.shelves.removeConfirmInline') }}</span>
                <button
                  class="shelf-modal-danger"
                  type="button"
                  :disabled="removingShelfID === shelf.id"
                  @click="onRemoveShelf(shelf)"
                >
                  {{ removingShelfID === shelf.id ? t('settings.shelves.removing') : t('settings.shelves.removeConfirmYes') }}
                </button>
                <button
                  class="shelf-modal-secondary"
                  type="button"
                  :disabled="removingShelfID === shelf.id"
                  @click="confirmingShelfID = ''"
                >
                  {{ t('common.cancel') }}
                </button>
              </div>

              <button
                v-else
                class="shelf-modal-remove"
                type="button"
                :disabled="operationInProgress"
                @click="confirmingShelfID = shelf.id"
              >
                {{ t('settings.shelves.remove') }}
              </button>
            </li>
          </ul>
        </section>

        <form class="shelf-modal-form" @submit.prevent="onSubmitAddShelf">
          <h3>{{ t('shelfModal.createTitle') }}</h3>
          <div class="shelf-modal-fields">
            <input
              v-model="newShelfName"
              class="shelf-modal-input"
              type="text"
              :placeholder="t('settings.shelves.addShelfNamePlaceholder')"
              :disabled="operationInProgress"
            />
            <div class="shelf-modal-directory-row">
              <input
                v-model="newShelfDirectory"
                class="shelf-modal-input"
                type="text"
                :placeholder="t('settings.shelves.addShelfDirectoryPlaceholder')"
                :disabled="operationInProgress"
              />
              <button
                class="shelf-modal-secondary"
                type="button"
                :disabled="operationInProgress"
                @click="onBrowseShelfDirectory"
              >
                {{ t('settings.shelves.addShelfBrowse') }}
              </button>
            </div>
          </div>
          <footer class="shelf-modal-actions">
            <button class="button" type="button" :disabled="operationInProgress" @click="requestClose">
              {{ t('common.cancel') }}
            </button>
            <button class="create-layer-submit" type="submit" :disabled="operationInProgress || !canSubmitAddShelf">
              {{ addingShelf ? t('settings.shelves.addShelfAdding') : t('settings.shelves.addShelfSubmit') }}
            </button>
          </footer>
        </form>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { ShelfInfo } from '../api/shelves';
import { useI18n } from '../i18n';
import { getBookshelfProvider } from '../providers';

const props = defineProps<{
  open: boolean;
  shelves: ShelfInfo[];
  loading?: boolean;
  error?: string;
}>();

const emit = defineEmits<{
  close: [];
  refresh: [];
}>();

const { t } = useI18n();
const titleId = 'shelf-management-modal-title';
const newShelfName = ref('');
const newShelfDirectory = ref('');
const addShelfError = ref('');
const removeShelfError = ref('');
const addingShelf = ref(false);
const removingShelfID = ref('');
const confirmingShelfID = ref('');

const canSubmitAddShelf = computed(() => newShelfName.value.trim().length > 0 && newShelfDirectory.value.trim().length > 0);
const operationInProgress = computed(() => addingShelf.value || removingShelfID.value.length > 0);
const errorMessage = computed(() => addShelfError.value || removeShelfError.value || props.error || '');

function resetForm(): void {
  newShelfName.value = '';
  newShelfDirectory.value = '';
  addShelfError.value = '';
  removeShelfError.value = '';
  confirmingShelfID.value = '';
}

function requestClose(): void {
  if (operationInProgress.value) {
    return;
  }
  emit('close');
}

function onBackdropClick(): void {
  requestClose();
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
  if (!name || !dir) {
    return;
  }

  addingShelf.value = true;
  addShelfError.value = '';
  removeShelfError.value = '';

  try {
    const provider = getBookshelfProvider();
    await provider.addDesktopShelf!(name, dir);
    newShelfName.value = '';
    newShelfDirectory.value = '';
    emit('refresh');
  } catch (err) {
    addShelfError.value = err instanceof Error ? err.message : t('settings.shelves.addShelfFailed');
  } finally {
    addingShelf.value = false;
  }
}

async function onRemoveShelf(shelf: ShelfInfo): Promise<void> {
  const provider = getBookshelfProvider();
  if (!provider.removeDesktopShelf || removingShelfID.value) {
    return;
  }

  removingShelfID.value = shelf.id;
  addShelfError.value = '';
  removeShelfError.value = '';

  try {
    await provider.removeDesktopShelf(shelf.id);
    confirmingShelfID.value = '';
    emit('refresh');
  } catch (err) {
    removeShelfError.value = err instanceof Error ? err.message : t('settings.shelves.removeFailed');
  } finally {
    removingShelfID.value = '';
  }
}

watch(
  () => props.open,
  () => resetForm()
);
</script>

<style scoped>
.shelf-modal-overlay {
  align-items: center;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 24px;
  position: fixed;
  z-index: 1000;
}

.shelf-modal {
  display: grid;
  gap: 18px;
  max-height: min(760px, calc(100vh - 48px));
  max-width: 720px;
  overflow: auto;
  width: min(720px, 100%);
}

.shelf-modal-header,
.shelf-modal-actions,
.shelf-modal-item,
.shelf-modal-confirm,
.shelf-modal-directory-row {
  align-items: center;
  display: flex;
  gap: 10px;
}

.shelf-modal-header,
.shelf-modal-actions,
.shelf-modal-item {
  justify-content: space-between;
}

.shelf-modal-header h2,
.shelf-modal-form h3,
.shelf-modal-header p {
  margin: 0;
}

.shelf-modal-header p,
.shelf-modal-muted,
.shelf-modal-item-copy span {
  color: var(--muted);
}

.shelf-modal-close {
  background: transparent;
  border: 0;
  color: var(--muted);
  cursor: pointer;
  font-size: 28px;
  line-height: 1;
}

.shelf-modal-close:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.shelf-modal-list {
  display: grid;
  gap: 8px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.shelf-modal-item {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 12px;
}

.shelf-modal-item-copy {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.shelf-modal-item-copy span {
  font-size: 12px;
  overflow-wrap: anywhere;
}

.shelf-modal-form,
.shelf-modal-fields {
  display: grid;
  gap: 12px;
}

.shelf-modal-input {
  border: 1px solid var(--border);
  border-radius: 10px;
  font: inherit;
  padding: 10px 12px;
  width: 100%;
}

.shelf-modal-input:disabled {
  background: #eef2f7;
}

.shelf-modal-directory-row .shelf-modal-input {
  flex: 1;
}

.shelf-modal-secondary,
.shelf-modal-remove,
.shelf-modal-danger {
  border: 1px solid var(--border);
  border-radius: 999px;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  padding: 7px 12px;
}

.shelf-modal-secondary {
  background: #f8fafc;
  color: #334155;
}

.shelf-modal-remove,
.shelf-modal-danger {
  background: #fff1f2;
  border-color: #fecdd3;
  color: #be123c;
}

.shelf-modal-message {
  border-radius: 10px;
  margin: 0;
  padding: 10px 12px;
}

.shelf-modal-message-error {
  background: #fff1f2;
  color: #be123c;
}

@media (max-width: 640px) {
  .shelf-modal-item,
  .shelf-modal-confirm,
  .shelf-modal-directory-row,
  .shelf-modal-actions {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
