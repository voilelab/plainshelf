<template>
  <BaseDialog
    :open="open"
    :title="t('layout.transferFolder.title')"
    :busy="busy"
    @close="emit('close')"
  >
    <section class="panel transfer-modal">
      <h2>{{ t('layout.transferFolder.title') }}</h2>

      <template v-if="!started">
        <p class="transfer-intro">{{ t('layout.transferFolder.description', { folder: folderName }) }}</p>

        <p v-if="destinationShelves.length === 0" class="transfer-empty">
          {{ shelvesError || t('layout.transferFolder.noShelves') }}
        </p>

        <template v-else>
          <label class="transfer-field">
            <span>{{ t('layout.transferFolder.shelfLabel') }}</span>
            <select v-model="targetShelfId" class="input">
              <option value="" disabled>{{ t('layout.transferFolder.chooseShelf') }}</option>
              <option v-for="shelf in destinationShelves" :key="shelf.id" :value="shelf.id">
                {{ shelf.name }}
              </option>
            </select>
          </label>

          <label class="transfer-field">
            <span>{{ t('layout.transferFolder.parentLabel') }}</span>
            <select v-model="targetParentFolder" class="input" :disabled="!targetShelfId || loadingFolders">
              <option value="/">{{ t('layout.transferFolder.rootFolder') }}</option>
              <option v-for="folder in shelfFolders" :key="folder" :value="folder">{{ folder }}</option>
            </select>
            <span v-if="loadingFolders" class="transfer-hint">{{ t('layout.transferFolder.loadingFolders') }}</span>
            <span v-else-if="folderError" class="transfer-error" role="alert">{{ folderError }}</span>
            <span v-else class="transfer-hint">
              {{ t('layout.transferFolder.parentHint', { destination: destinationPreview }) }}
            </span>
          </label>

          <fieldset class="transfer-field transfer-modes">
            <legend>{{ t('layout.transferFolder.modeLabel') }}</legend>
            <label class="transfer-mode">
              <input v-model="mode" type="radio" value="copy" />
              <span class="transfer-mode-text">
                <span class="transfer-mode-name">{{ t('layout.transferFolder.modeCopy') }}</span>
                <span class="transfer-mode-hint">{{ t('layout.transferFolder.modeCopyHint') }}</span>
              </span>
            </label>
            <label v-if="moveAvailable" class="transfer-mode">
              <input v-model="mode" type="radio" value="move" />
              <span class="transfer-mode-text">
                <span class="transfer-mode-name">{{ t('layout.transferFolder.modeMove') }}</span>
                <span class="transfer-mode-hint">{{ t('layout.transferFolder.modeMoveHint') }}</span>
              </span>
            </label>
            <p v-else class="transfer-hint">{{ t('layout.transferFolder.readOnlySource') }}</p>
          </fieldset>
        </template>

        <p v-if="error" class="transfer-error" role="alert">{{ error }}</p>

        <footer>
          <button type="button" class="button" :disabled="busy" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="button primary"
            :disabled="busy || !canSubmit"
            @click="submit"
          >
            {{ t('layout.transferFolder.confirm') }}
          </button>
        </footer>
      </template>

      <template v-else>
        <p class="transfer-status">{{ statusText }}</p>
        <ProgressBar :value="percentage" :label="t('layout.transferFolder.progressLabel')" />
        <p class="transfer-progress-value">
          <span>{{ Math.round(percentage) }}%</span>
          <span v-if="counts" class="transfer-progress-count">
            {{ t('layout.transferFolder.progressCount', { done: counts.processed, total: counts.total }) }}
            <template v-if="counts.failed > 0">
              · {{ t('layout.transferFolder.failedCount', { failed: counts.failed }) }}
            </template>
          </span>
        </p>
        <p v-if="error" class="transfer-error" role="alert">{{ error }}</p>
        <footer>
          <button type="button" class="button primary" :disabled="!finished" @click="emit('close')">
            {{ t('layout.transferFolder.close') }}
          </button>
        </footer>
      </template>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import BaseDialog from '@/components/BaseDialog.vue';
import ProgressBar from '@/components/ProgressBar.vue';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { bookshelfWriter } from '@/providers';
import type { BookTransferMode } from '@/api/books';
import { folderTransferCounts, type TaskChain, type TaskStatus } from '@/types/task';
import { useI18n } from '@/i18n';

const props = defineProps<{
  open: boolean;
  folderName: string;
  busy: boolean;
  started: boolean;
  finished: boolean;
  status: TaskStatus;
  percentage: number;
  chain: TaskChain | null;
  error?: string;
}>();

const emit = defineEmits<{
  close: [];
  submit: [payload: { targetShelfId: string; targetParentFolder: string; mode: BookTransferMode }];
}>();

const { t } = useI18n();
const {
  transferDestinationShelves: destinationShelves,
  selectedShelfReadOnly,
  error: shelvesError,
  ensureShelvesLoaded
} = useShelvesStore();

const targetShelfId = ref('');
const targetParentFolder = ref('/');
const mode = ref<BookTransferMode>('copy');
const shelfFolders = ref<string[]>([]);
const loadingFolders = ref(false);
const folderError = ref('');
let folderRequest = 0;

const moveAvailable = computed(() => !selectedShelfReadOnly.value);
const effectiveMode = computed<BookTransferMode>(() => (moveAvailable.value ? mode.value : 'copy'));

const canSubmit = computed(() => targetShelfId.value !== '' && !loadingFolders.value);

const destinationPreview = computed(() =>
  targetParentFolder.value === '/' ? props.folderName : `${targetParentFolder.value}/${props.folderName}`
);

const counts = computed(() => folderTransferCounts(props.chain));

const statusText = computed(() => {
  switch (props.status) {
    case 'completed':
      return effectiveMode.value === 'move'
        ? t('layout.transferFolder.completedMove')
        : t('layout.transferFolder.completedCopy');
    case 'partially_completed':
      return t('layout.transferFolder.partial');
    case 'failed':
      return t('layout.transferFolder.failed');
    case 'running':
      return t('layout.transferFolder.running');
    default:
      return t('layout.transferFolder.pending');
  }
});

async function loadFolders(shelfID: string): Promise<void> {
  const request = (folderRequest += 1);
  loadingFolders.value = true;
  folderError.value = '';
  shelfFolders.value = [];
  try {
    const folders = await bookshelfWriter().listShelfFolders(shelfID);
    if (request !== folderRequest) {
      return;
    }
    shelfFolders.value = folders.filter((folder) => folder && folder !== '/');
  } catch (err) {
    if (request !== folderRequest) {
      return;
    }
    folderError.value = err instanceof Error ? err.message : t('layout.transferFolder.foldersFailed');
  } finally {
    if (request === folderRequest) {
      loadingFolders.value = false;
    }
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }
    targetShelfId.value = '';
    targetParentFolder.value = '/';
    mode.value = 'copy';
    shelfFolders.value = [];
    folderError.value = '';
    folderRequest += 1;
    void ensureShelvesLoaded();
  }
);

watch(targetShelfId, (shelfID) => {
  targetParentFolder.value = '/';
  shelfFolders.value = [];
  folderError.value = '';
  if (shelfID) {
    void loadFolders(shelfID);
  } else {
    folderRequest += 1;
  }
});

function submit(): void {
  if (!canSubmit.value) {
    return;
  }
  emit('submit', {
    targetShelfId: targetShelfId.value,
    targetParentFolder: targetParentFolder.value === '/' ? '' : targetParentFolder.value,
    mode: effectiveMode.value
  });
}
</script>

<style scoped>
.transfer-modal {
  display: grid;
  gap: 16px;
  max-width: 460px;
  padding: 18px;
  width: min(100%, 460px);
}

.transfer-modal h2 {
  margin: 0;
}

.transfer-intro {
  color: #475569;
  margin: 0;
}

.transfer-empty {
  color: #475569;
  margin: 0;
}

.transfer-field {
  display: grid;
  gap: 6px;
}

.transfer-modes {
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 10px;
  margin: 0;
  padding: 12px;
}

.transfer-modes legend {
  font-weight: 600;
  padding: 0 4px;
}

.transfer-mode {
  align-items: start;
  display: flex;
  gap: 10px;
  padding: 6px 0;
}

.transfer-mode-text {
  display: grid;
  gap: 2px;
}

.transfer-mode-name {
  font-weight: 600;
}

.transfer-mode-hint {
  color: #64748b;
  font-size: 13px;
}

.transfer-hint {
  color: #64748b;
  font-size: 13px;
}

.transfer-status {
  margin: 0;
}

.transfer-progress-value {
  align-items: baseline;
  color: #64748b;
  display: flex;
  font-size: 13px;
  gap: 8px;
  justify-content: space-between;
  margin: 0;
}

.transfer-progress-count {
  text-align: right;
}

.transfer-error {
  color: #991b1b;
  font-size: 13px;
  margin: 0;
}

.transfer-modal footer {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
</style>
