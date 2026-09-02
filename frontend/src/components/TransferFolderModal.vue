<template>
  <BaseDialog
    :open="open"
    :title="t('layout.transferFolder.title')"
    :busy="busy"
    @close="emit('close')"
  >
    <section class="panel transfer-modal">
      <h2>{{ t('layout.transferFolder.title') }}</h2>

      <!-- Once the chain is scheduled the form is replaced by its progress, the
           same shape the single-book transfer uses. -->
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
          <!-- The chain keeps running on the server regardless, so the only close
               is offered once it settles — mirroring the single-book transfer. -->
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

// The progress-related props are driven by the useTaskChainProgress instance the
// parent owns (through useFolderManagement): the modal renders the chain, it does
// not poll it. `started` splits the form from the progress view; `busy` is the
// running flag that keeps the dialog from being dismissed mid-transfer. `chain`
// is passed so the per-book "N of M" count can be read from its task result,
// which the bare percentage does not carry.
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
// '/' is the labelled root option; it maps to '' (the shelf root) on submit.
const targetParentFolder = ref('/');
const mode = ref<BookTransferMode>('copy');
const shelfFolders = ref<string[]>([]);
const loadingFolders = ref(false);
const folderError = ref('');
// Bumped on every shelf change so a slow folder fetch for a shelf the user has
// since switched away from cannot overwrite the current one's folders.
let folderRequest = 0;

// A move ends by deleting the source, so a read-only shelf offers only the
// copy — which the server allows, because only the target is written
// (server/handle_book_transfers.go, server/handle_folder_transfers.go).
const moveAvailable = computed(() => !selectedShelfReadOnly.value);
const effectiveMode = computed<BookTransferMode>(() =>
  moveAvailable.value ? mode.value : 'copy'
);

const canSubmit = computed(() => targetShelfId.value !== '' && !loadingFolders.value);

// The folder keeps its own name and nests under the chosen parent, so preview the
// path it will land at on the target shelf.
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
    // A newer selection (or a close) landed while this was in flight.
    if (request !== folderRequest) {
      return;
    }
    // The root is offered as its own option, so drop it from the list.
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
    // Reset the form each time it opens so a previous attempt's picks do not
    // carry over.
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
