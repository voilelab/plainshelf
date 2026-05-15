<template>
  <div class="layout-root">
    <DeleteModal
      :open="pendingDeleteLayerPath.length > 0"
      title="Delete layer"
      :item-name="pendingDeleteLayerPath"
      description="This will fail if the layer contains books or child layers."
      :busy="isDeletingPendingLayer"
      :error="deleteLayerError"
      @cancel="cancelPendingDeleteLayer"
      @confirm="confirmDeleteLayer"
    />

    <aside class="sidebar" :class="{ collapsed: isCollapsed }">
      <button
        class="collapse-btn"
        type="button"
        :aria-label="isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
        @click="isCollapsed = !isCollapsed"
      >
        {{ isCollapsed ? '→' : '←' }}
      </button>

      <div v-if="!isCollapsed" class="sidebar-inner">
        <section class="sidebar-section" aria-label="Layers">
          <div class="sidebar-header-row">
            <div class="sidebar-section-title">LAYERS</div>
            <button
              type="button"
              class="create-layer-toggle"
              :disabled="creatingLayer"
              @click="toggleCreateLayerForm"
            >
              {{ showCreateLayerForm ? 'Cancel' : '新增 Layer' }}
            </button>
          </div>

          <form v-if="showCreateLayerForm" class="create-layer-form" @submit.prevent="onSubmitCreateLayer">
            <input
              v-model="newLayerPath"
              class="create-layer-input"
              type="text"
              placeholder="例如 programming/rust"
              :disabled="creatingLayer"
            >
            <div class="create-layer-actions">
              <button
                type="submit"
                class="create-layer-submit"
                :disabled="creatingLayer || !canSubmitCreateLayer"
              >
                {{ creatingLayer ? '建立中...' : '建立' }}
              </button>
            </div>
          </form>

          <p v-if="createLayerError" class="sidebar-error" role="alert">
            {{ createLayerError }}
          </p>
          <p v-if="createLayerSuccess" class="sidebar-success" role="status">
            {{ createLayerSuccess }}
            <button
              v-if="createdLayerPath"
              type="button"
              class="success-action"
              @click="enterCreatedLayer"
            >
              進入
            </button>
          </p>

          <div v-if="layersLoading" class="sidebar-status">Loading layers...</div>
          <div v-else-if="layersError" class="sidebar-status sidebar-error sidebar-layer-error" role="alert">
            <p>{{ layersError }}</p>
            <button type="button" class="button" @click="fetchLayers">Retry</button>
          </div>
          <LayerTree
            v-else
            :nodes="layerTree"
            :selected="currentLayer"
            :deleting-map="deletingLayerMap"
            @select="onSelectLayer"
            @move-book="onMoveBook"
            @delete-layer="requestDeleteLayer"
          />
        </section>
        <p v-if="moveBookError" class="sidebar-error" role="alert">
          {{ moveBookError }}
        </p>
        <p v-if="deleteLayerError && !pendingDeleteLayerPath" class="sidebar-error sidebar-error-pre" role="alert">
          {{ deleteLayerError }}
        </p>

        <div class="sidebar-nav-divider" role="presentation"></div>

        <section class="sidebar-section" aria-label="Maintenance">
          <div class="sidebar-section-title">MAINTENANCE</div>
          <nav class="sidebar-nav-list" aria-label="Maintenance links">
            <RouterLink
              v-for="item in MAINTENANCE_NAV_ITEMS"
              :key="item.key"
              :to="item.to"
              class="sidebar-nav-item"
              exact-active-class="active"
            >
              {{ item.label }}
            </RouterLink>
          </nav>
        </section>
      </div>
    </aside>

    <main class="main-content">
      <header class="topbar">
        <h1 class="brand">
          <img class="brand-icon" :src="appIcon" alt="" aria-hidden="true">
          <span>PlainShelf</span>
        </h1>
      </header>

      <div class="page-area">
        <RouterView />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import DeleteModal from '../components/DeleteModal.vue';
import LayerTree from '../components/LayerTree.vue';
import { updateBookLayer } from '../api/books';
import { createLayer, deleteLayer } from '../api/layers';
import { useBookStore } from '../composables/useBookStore';
import { useLayerStore } from '../composables/useLayerStore';
import { buildLayerTreeNodes, getLayerPath, normalizeLayerPath } from '../utils/layers';
import { MAINTENANCE_NAV_ITEMS } from '../utils/maintenance';
import appIcon from '../assets/icon-192.png';

const isCollapsed = ref(false);
const route = useRoute();
const router = useRouter();
const { books, loading, fetchBooks } = useBookStore();
const { layers, loading: layersLoading, error: layersError, loaded: layersLoaded, fetchLayers } = useLayerStore();
const moveBookError = ref('');
const showCreateLayerForm = ref(false);
const creatingLayer = ref(false);
const createLayerError = ref('');
const createLayerSuccess = ref('');
const createdLayerPath = ref('');
const newLayerPath = ref('');
const deleteLayerError = ref('');
const deletingLayerMap = ref<Record<string, boolean>>({});
const pendingDeleteLayerPath = ref('');

const currentLayer = computed(() => {
  const q = route.query.layers;
  return typeof q === 'string' && q.length > 0 ? q : undefined;
});

const layerTree = computed(() => buildLayerTreeNodes(layers.value));
const canSubmitCreateLayer = computed(() => normalizeLayerPath(newLayerPath.value).length > 0);
const isDeletingPendingLayer = computed(
  () => pendingDeleteLayerPath.value.length > 0 && (deletingLayerMap.value[pendingDeleteLayerPath.value] ?? false)
);

function goToLayer(layer: string | undefined): void {
  const query: Record<string, string> = { page: '1' };
  if (layer) query.layers = layer;
  void router.push({ path: '/books', query });
}

function normalizeLayerSelectionPath(path: string): string | undefined {
  const trimmed = path.trim();
  if (trimmed === '') {
    return undefined;
  }
  if (trimmed === '/') {
    return '/';
  }

  const normalized = normalizeLayerPath(trimmed);
  return normalized.length > 0 ? normalized : undefined;
}

function onSelectLayer(path: string): void {
  deleteLayerError.value = '';
  goToLayer(normalizeLayerSelectionPath(path));
}

function toggleCreateLayerForm(): void {
  showCreateLayerForm.value = !showCreateLayerForm.value;
  createLayerError.value = '';
  createLayerSuccess.value = '';

  if (!showCreateLayerForm.value) {
    newLayerPath.value = '';
  }
}

async function onSubmitCreateLayer(): Promise<void> {
  const normalized = normalizeLayerPath(newLayerPath.value);
  if (!normalized) {
    createLayerError.value = 'Layer path cannot be empty';
    createLayerSuccess.value = '';
    return;
  }

  creatingLayer.value = true;
  createLayerError.value = '';
  createLayerSuccess.value = '';

  try {
    await createLayer(normalized);
    await fetchLayers();

    createdLayerPath.value = normalized;
    createLayerSuccess.value = 'Layer created';
    newLayerPath.value = '';
    showCreateLayerForm.value = false;
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Failed to create layer';

    if (message === 'Layer path cannot be empty') {
      createLayerError.value = 'Layer path cannot be empty';
    } else if (message === 'Failed to create layer') {
      createLayerError.value = 'Failed to create layer';
    } else {
      createLayerError.value = message || 'Failed to create layer';
    }
  } finally {
    creatingLayer.value = false;
  }
}

function enterCreatedLayer(): void {
  if (!createdLayerPath.value) {
    return;
  }

  goToLayer(createdLayerPath.value);
  createLayerSuccess.value = '';
}

async function onMoveBook(payload: { bookId: string; targetLayer: string }): Promise<void> {
  moveBookError.value = '';

  const currentBook = books.value.find((item) => item.id === payload.bookId);
  if (!currentBook) {
    moveBookError.value = 'Book not found.';
    return;
  }

  const currentLayerPath = getLayerPath(currentBook);
  if (currentLayerPath === payload.targetLayer) {
    return;
  }

  try {
    await updateBookLayer(payload.bookId, payload.targetLayer);
    await fetchBooks();
  } catch (err) {
    moveBookError.value = err instanceof Error ? err.message : 'Failed to move book.';
  }
}

function requestDeleteLayer(path: string): void {
  if (deletingLayerMap.value[path]) {
    return;
  }

  deleteLayerError.value = '';
  pendingDeleteLayerPath.value = path;
}

function cancelPendingDeleteLayer(): void {
  if (isDeletingPendingLayer.value) {
    return;
  }

  pendingDeleteLayerPath.value = '';
  deleteLayerError.value = '';
}

async function confirmDeleteLayer(): Promise<void> {
  const path = pendingDeleteLayerPath.value;
  if (!path || deletingLayerMap.value[path]) {
    return;
  }

  deleteLayerError.value = '';
  deletingLayerMap.value = {
    ...deletingLayerMap.value,
    [path]: true
  };

  try {
    await deleteLayer(path);
    await Promise.all([fetchLayers(), fetchBooks()]);

    if (currentLayer.value === path) {
      goToLayer(undefined);
    }

    pendingDeleteLayerPath.value = '';
  } catch (err) {
    const message = err instanceof Error ? err.message : '';
    if (message === 'Cannot delete this layer because it is not empty.') {
      deleteLayerError.value =
        'Cannot delete this layer because it is not empty.\nMove books out and delete child layers first.';
    } else if (message) {
      deleteLayerError.value = message;
    } else {
      deleteLayerError.value = 'Failed to delete layer';
    }
  } finally {
    const { [path]: _deleted, ...rest } = deletingLayerMap.value;
    deletingLayerMap.value = rest;
  }
}

onMounted(async () => {
  if (!layersLoaded.value && !layersLoading.value) {
    await fetchLayers();
  }

  if (books.value.length === 0 && !loading.value) {
    await fetchBooks();
  }
});
</script>

<style scoped>
.layout-root {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
}

/* ── Sidebar ── */
.sidebar {
  width: 240px;
  min-width: 200px;
  max-width: 300px;
  border-right: 1px solid var(--border);
  position: sticky;
  top: 0;
  height: 100vh;
  overflow-y: auto;
  background: linear-gradient(180deg, #e9edf2 0%, #e3e8ef 100%);
  backdrop-filter: blur(8px);
  transition: width 0.2s ease;
  flex-shrink: 0;
}

.sidebar.collapsed {
  width: 40px;
  min-width: 0;
  overflow: hidden;
}

.collapse-btn {
  align-items: center;
  background: #f6f9fc;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: #3e4e66;
  cursor: pointer;
  display: flex;
  font-size: 12px;
  font-weight: 700;
  height: 24px;
  justify-content: center;
  margin: 12px auto 0;
  width: 24px;
}

.collapse-btn:hover {
  background: #ecf2f9;
}

.sidebar-inner {
  padding: 8px;
}

.sidebar-header-row {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.create-layer-toggle {
  background: #f1f5f9;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 8px;
}

.create-layer-toggle:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.sidebar-layer-error {
  display: grid;
  gap: 8px;
  margin: 4px 8px;
}

.sidebar-layer-error p {
  margin: 0;
}

.sidebar-layer-error .button {
  justify-self: start;
}

.create-layer-form {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0 8px 8px;
}

.create-layer-input {
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 13px;
  line-height: 1.3;
  padding: 6px 8px;
}

.create-layer-actions {
  display: flex;
  justify-content: flex-end;
}

.create-layer-submit {
  background: #2563eb;
  border: 1px solid #1d4ed8;
  border-radius: 6px;
  color: #ffffff;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 10px;
}

.create-layer-submit:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.sidebar-error {
  color: #b91c1c;
  font-size: 12px;
  line-height: 1.4;
  margin: 8px 8px 0;
}

.sidebar-error-pre {
  white-space: pre-line;
}

.sidebar-success {
  align-items: center;
  color: #166534;
  display: flex;
  font-size: 12px;
  gap: 8px;
  line-height: 1.4;
  margin: 8px 8px 0;
}

.success-action {
  background: transparent;
  border: 1px solid #86efac;
  border-radius: 6px;
  color: #166534;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
}

.sidebar-status {
  color: #4f5d72;
  font-size: 12px;
  line-height: 1.4;
  margin: 2px 8px 0;
}

/* ── Main content ── */
.main-content {
  flex: 1;
  height: 100vh;
  overflow-y: auto;
  max-width: none;
  background: white;
  min-width: 0;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  background: rgba(255, 255, 255, 0.92);
  border-bottom: 1px solid var(--border);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 24px;
}

.brand {
  align-items: center;
  display: inline-flex;
  gap: 8px;
  margin: 0;
  font-size: 20px;
  letter-spacing: 0.3px;
}

.brand-icon {
  width: 20px;
  height: 20px;
  display: block;
}

.top-nav {
  display: flex;
  align-items: center;
  gap: 14px;
}

.top-link {
  color: var(--accent);
  font-weight: 600;
}

.page-area {
  padding: 16px 24px;
}
</style>
