<template>
  <BaseDialog
    :open="open"
    :title="t('layout.transferLayer.title')"
    :busy="busy"
    @close="emit('close')"
  >
    <section class="panel transfer-modal">
      <h2>{{ t('layout.transferLayer.title') }}</h2>

      <!-- Once the chain is scheduled the form is replaced by its progress, the
           same shape the single-book transfer uses. -->
      <template v-if="!started">
        <p class="transfer-intro">{{ t('layout.transferLayer.description', { folder: folderName }) }}</p>

        <p v-if="destinationShelves.length === 0" class="transfer-empty">
          {{ shelvesError || t('layout.transferLayer.noShelves') }}
        </p>

        <template v-else>
          <label class="transfer-field">
            <span>{{ t('layout.transferLayer.shelfLabel') }}</span>
            <select v-model="targetShelfId" class="input">
              <option value="" disabled>{{ t('layout.transferLayer.chooseShelf') }}</option>
              <option v-for="shelf in destinationShelves" :key="shelf.id" :value="shelf.id">
                {{ shelf.name }}
              </option>
            </select>
          </label>

          <label class="transfer-field">
            <span>{{ t('layout.transferLayer.parentLabel') }}</span>
            <select v-model="targetParentLayer" class="input" :disabled="!targetShelfId || loadingLayers">
              <option value="/">{{ t('layout.transferLayer.rootLayer') }}</option>
              <option v-for="layer in shelfLayers" :key="layer" :value="layer">{{ layer }}</option>
            </select>
            <span v-if="loadingLayers" class="transfer-hint">{{ t('layout.transferLayer.loadingLayers') }}</span>
            <span v-else-if="layerError" class="transfer-error" role="alert">{{ layerError }}</span>
            <span v-else class="transfer-hint">
              {{ t('layout.transferLayer.parentHint', { destination: destinationPreview }) }}
            </span>
          </label>

          <fieldset class="transfer-field transfer-modes">
            <legend>{{ t('layout.transferLayer.modeLabel') }}</legend>
            <label class="transfer-mode">
              <input v-model="mode" type="radio" value="copy" />
              <span class="transfer-mode-text">
                <span class="transfer-mode-name">{{ t('layout.transferLayer.modeCopy') }}</span>
                <span class="transfer-mode-hint">{{ t('layout.transferLayer.modeCopyHint') }}</span>
              </span>
            </label>
            <label class="transfer-mode">
              <input v-model="mode" type="radio" value="move" />
              <span class="transfer-mode-text">
                <span class="transfer-mode-name">{{ t('layout.transferLayer.modeMove') }}</span>
                <span class="transfer-mode-hint">{{ t('layout.transferLayer.modeMoveHint') }}</span>
              </span>
            </label>
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
            {{ t('layout.transferLayer.confirm') }}
          </button>
        </footer>
      </template>

      <template v-else>
        <p class="transfer-status">{{ statusText }}</p>
        <ProgressBar :value="percentage" :label="t('layout.transferLayer.progressLabel')" />
        <p class="transfer-progress-value">
          <span>{{ Math.round(percentage) }}%</span>
          <span v-if="counts" class="transfer-progress-count">
            {{ t('layout.transferLayer.progressCount', { done: counts.processed, total: counts.total }) }}
            <template v-if="counts.failed > 0">
              · {{ t('layout.transferLayer.failedCount', { failed: counts.failed }) }}
            </template>
          </span>
        </p>
        <p v-if="error" class="transfer-error" role="alert">{{ error }}</p>
        <footer>
          <!-- The chain keeps running on the server regardless, so the only close
               is offered once it settles — mirroring the single-book transfer. -->
          <button type="button" class="button primary" :disabled="!finished" @click="emit('close')">
            {{ t('layout.transferLayer.close') }}
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
import { layerTransferCounts, type TaskChain, type TaskStatus } from '@/types/task';
import { useI18n } from '@/i18n';

// The progress-related props are driven by the useTaskChainProgress instance the
// parent owns (through useLayerManagement): the modal renders the chain, it does
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
  submit: [payload: { targetShelfId: string; targetParentLayer: string; mode: BookTransferMode }];
}>();

const { t } = useI18n();
const { shelves, selectedShelfID, error: shelvesError, ensureShelvesLoaded } = useShelvesStore();

const targetShelfId = ref('');
// '/' is the labelled root option; it maps to '' (the shelf root) on submit.
const targetParentLayer = ref('/');
const mode = ref<BookTransferMode>('copy');
const shelfLayers = ref<string[]>([]);
const loadingLayers = ref(false);
const layerError = ref('');
// Bumped on every shelf change so a slow layer fetch for a shelf the user has
// since switched away from cannot overwrite the current one's layers.
let layerRequest = 0;

// The source shelf is the active one, and naming it twice is rejected by the
// server, so it is dropped from the destinations.
const destinationShelves = computed(() =>
  shelves.value.filter((shelf) => shelf.id !== selectedShelfID.value)
);

const canSubmit = computed(() => targetShelfId.value !== '' && !loadingLayers.value);

// The folder keeps its own name and nests under the chosen parent, so preview the
// path it will land at on the target shelf.
const destinationPreview = computed(() =>
  targetParentLayer.value === '/' ? props.folderName : `${targetParentLayer.value}/${props.folderName}`
);

const counts = computed(() => layerTransferCounts(props.chain));

const statusText = computed(() => {
  switch (props.status) {
    case 'completed':
      return mode.value === 'move'
        ? t('layout.transferLayer.completedMove')
        : t('layout.transferLayer.completedCopy');
    case 'partially_completed':
      return t('layout.transferLayer.partial');
    case 'failed':
      return t('layout.transferLayer.failed');
    case 'running':
      return t('layout.transferLayer.running');
    default:
      return t('layout.transferLayer.pending');
  }
});

async function loadLayers(shelfID: string): Promise<void> {
  const request = (layerRequest += 1);
  loadingLayers.value = true;
  layerError.value = '';
  shelfLayers.value = [];
  try {
    const layers = await bookshelfWriter().listShelfLayers(shelfID);
    // A newer selection (or a close) landed while this was in flight.
    if (request !== layerRequest) {
      return;
    }
    // The root is offered as its own option, so drop it from the list.
    shelfLayers.value = layers.filter((layer) => layer && layer !== '/');
  } catch (err) {
    if (request !== layerRequest) {
      return;
    }
    layerError.value = err instanceof Error ? err.message : t('layout.transferLayer.layersFailed');
  } finally {
    if (request === layerRequest) {
      loadingLayers.value = false;
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
    targetParentLayer.value = '/';
    mode.value = 'copy';
    shelfLayers.value = [];
    layerError.value = '';
    layerRequest += 1;
    void ensureShelvesLoaded();
  }
);

watch(targetShelfId, (shelfID) => {
  targetParentLayer.value = '/';
  shelfLayers.value = [];
  layerError.value = '';
  if (shelfID) {
    void loadLayers(shelfID);
  } else {
    layerRequest += 1;
  }
});

function submit(): void {
  if (!canSubmit.value) {
    return;
  }
  emit('submit', {
    targetShelfId: targetShelfId.value,
    targetParentLayer: targetParentLayer.value === '/' ? '' : targetParentLayer.value,
    mode: mode.value
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
