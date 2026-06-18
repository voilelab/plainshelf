<template>
  <div class="layout-root">
    <DeleteModal
      :open="pendingDeleteLayerPath.length > 0"
      :title="t('layout.deleteLayer.title')"
      :item-name="pendingDeleteLayerPath"
      :description="t('layout.deleteLayer.description')"
      :busy="isDeletingPendingLayer"
      :error="deleteLayerError"
      @cancel="cancelPendingDeleteLayer"
      @confirm="confirmDeleteLayer"
    />
    <RenameLayerModal
      :open="pendingRenameLayerPath.length > 0"
      :current-name="pendingRenameLayerName"
      :busy="isRenamingPendingLayer"
      :error="renameLayerError"
      @cancel="cancelPendingRenameLayer"
      @submit="confirmRenameLayer"
    />

    <aside class="sidebar" :class="{ collapsed: isCollapsed }">
      <button
        class="collapse-btn"
        type="button"
        :aria-label="isCollapsed ? t('layout.expandSidebar') : t('layout.collapseSidebar')"
        @click="isCollapsed = !isCollapsed"
      >
        {{ isCollapsed ? '→' : '←' }}
      </button>

      <div v-if="!isCollapsed" class="sidebar-inner">
        <section class="sidebar-section" :aria-label="t('layout.shelf.label')">
          <label class="sidebar-shelf-label">
            <span class="sidebar-section-title">{{ t('layout.shelf.label') }}</span>
            <select
              class="sidebar-shelf-select"
              :value="selectedShelfID"
              :disabled="shelvesLoading || shelves.length === 0"
              @change="onShelfChange"
            >
              <option v-if="shelvesLoading" value="">{{ t('layout.shelf.loading') }}</option>
              <option v-else-if="shelves.length === 0" value="">{{ t('layout.shelf.empty') }}</option>
              <option v-for="shelf in shelves" :key="shelf.id" :value="shelf.id">
                {{ shelf.name }}
              </option>
            </select>
          </label>
          <p v-if="shelvesError" class="sidebar-error" role="alert">{{ shelvesError }}</p>
        </section>

        <template v-if="hasActiveShelf">
          <div class="sidebar-nav-divider" role="presentation"></div>
          <section class="sidebar-section" :aria-label="t('layout.sections.layers')">
            <div class="sidebar-header-row">
              <div class="sidebar-section-title">{{ t('layout.sections.layers') }}</div>
              <button
                type="button"
                class="create-layer-toggle"
                :disabled="readOnly || creatingLayer"
                @click="toggleCreateLayerForm"
              >
                {{ showCreateLayerForm ? t('layout.createLayer.cancel') : t('layout.createLayer.add') }}
              </button>
            </div>

            <form v-if="showCreateLayerForm && !readOnly" class="create-layer-form" @submit.prevent="onSubmitCreateLayer">
              <input
                v-model="newLayerPath"
                class="create-layer-input"
                type="text"
                :placeholder="t('layout.createLayer.placeholder')"
                :disabled="creatingLayer"
              >
              <div class="create-layer-actions">
                <button
                  type="submit"
                  class="create-layer-submit"
                  :disabled="readOnly || creatingLayer || !canSubmitCreateLayer"
                >
                  {{ creatingLayer ? t('layout.createLayer.creating') : t('layout.createLayer.create') }}
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
                {{ t('layout.createLayer.enter') }}
              </button>
            </p>

            <div v-if="layersLoading" class="sidebar-status">{{ t('layout.createLayer.loadingLayers') }}</div>
            <div v-else-if="layersError" class="sidebar-status sidebar-error sidebar-layer-error" role="alert">
              <p>{{ layersError }}</p>
              <button type="button" class="button" @click="fetchLayers">{{ t('common.retry') }}</button>
            </div>
            <LayerTree
              v-else
              :nodes="layerTree"
              :selected="currentLayer"
              :deleting-map="deletingLayerMap"
              :read-only="readOnly"
              @select="onSelectLayer"
              @move-book="onMoveBook"
              @delete-layer="requestDeleteLayer"
              @rename-layer="requestRenameLayer"
              @move-layer="onMoveLayer"
            />
          </section>
          <p v-if="moveBookError" class="sidebar-error" role="alert">
            {{ moveBookError }}
          </p>
          <p v-if="deleteLayerError && !pendingDeleteLayerPath" class="sidebar-error sidebar-error-pre" role="alert">
            {{ deleteLayerError }}
          </p>
          <p v-if="layerOperationError" class="sidebar-error sidebar-error-pre" role="alert">
            {{ layerOperationError }}
          </p>

          <div class="sidebar-nav-divider" role="presentation"></div>

          <section class="sidebar-section" :aria-label="t('layout.sections.reading')">
            <div class="sidebar-section-title">{{ t('layout.sections.reading') }}</div>
            <nav class="sidebar-nav-list" :aria-label="t('layout.sections.reading')">
              <RouterLink
                to="/read-history"
                class="sidebar-nav-item"
                exact-active-class="active"
              >
                <SidebarNavIcon name="recently-read" />
                <span>{{ t('layout.recentlyRead') }}</span>
              </RouterLink>
              <RouterLink
                to="/trash"
                class="sidebar-nav-item"
                exact-active-class="active"
              >
                <SidebarNavIcon name="trash" />
                <span>{{ t('layout.trash') }}</span>
              </RouterLink>
            </nav>
          </section>

          <div class="sidebar-nav-divider" role="presentation"></div>

          <section class="sidebar-section" :aria-label="t('layout.sections.maintenance')">
            <div class="sidebar-section-title">{{ t('layout.sections.maintenance') }}</div>
            <nav class="sidebar-nav-list" :aria-label="t('layout.sections.maintenance')">
              <RouterLink
                v-for="item in MAINTENANCE_NAV_ITEMS"
                :key="item.key"
                :to="item.to"
                class="sidebar-nav-item"
                exact-active-class="active"
              >
                <SidebarNavIcon v-if="item.icon" :name="item.icon" />
                <span>{{ t(item.labelKey) }}</span>
              </RouterLink>
            </nav>
          </section>
        </template>

        <div class="sidebar-nav-divider" role="presentation"></div>

        <section class="sidebar-section" :aria-label="t('layout.sections.admin')">
          <div class="sidebar-section-title">{{ t('layout.sections.admin') }}</div>
          <nav class="sidebar-nav-list" :aria-label="t('layout.sections.admin')">
            <RouterLink v-if="hasActiveShelf" to="/admin/logs" class="sidebar-nav-item" exact-active-class="active">
              <SidebarNavIcon name="logs" />
              <span>{{ t('layout.adminLogs') }}</span>
            </RouterLink>
            <RouterLink to="/settings" class="sidebar-nav-item" exact-active-class="active">
              <SidebarNavIcon name="settings" />
              <span>{{ t('layout.settings') }}</span>
            </RouterLink>
          </nav>
        </section>
      </div>
    </aside>

    <main class="main-content">
      <div v-if="readOnly" class="read-only-banner" role="status">
        {{ t('layout.readOnly.banner') }}
      </div>
      <header class="topbar">
        <h1 class="brand">
          <img class="brand-icon" :src="appIcon" alt="" aria-hidden="true">
          <span>{{ t('app.name') }}</span>
        </h1>
        <div class="topbar-controls">
          <label class="language-select">
            <span>{{ t('language.label') }}</span>
            <select class="language-select-control" :value="locale" @change="onLocaleChange">
              <option v-for="lang in supportedLocales" :key="lang" :value="lang">
                {{ t(localeLabelKeyMap[lang]) }}
              </option>
            </select>
          </label>
        </div>
      </header>

      <div class="page-area">
        <RouterView v-if="canShowRouteContent" />
        <section v-else class="no-shelf-panel" role="status">
          <h2>{{ t('layout.shelf.unavailableTitle') }}</h2>
          <p>{{ shelfUnavailableMessage }}</p>
          <RouterLink to="/settings" class="button">{{ t('layout.settings') }}</RouterLink>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import DeleteModal from '../components/DeleteModal.vue';
import LayerTree from '../components/LayerTree.vue';
import RenameLayerModal from '../components/RenameLayerModal.vue';
import SidebarNavIcon from '../components/SidebarNavIcon.vue';
import { getBookshelfProvider } from '../providers';
import { createLayer, deleteLayer, moveLayer, renameLayer } from '../api/layers';
import { useBookStore } from '../composables/useBookStore';
import { useLayerStore } from '../composables/useLayerStore';
import { useShelvesStore } from '../composables/useShelvesStore';
import { useServerMode } from '../composables/useServerMode';
import { buildLayerTreeNodes, getLayerPath, normalizeLayerPath } from '../utils/layers';
import { MAINTENANCE_NAV_ITEMS } from '../utils/maintenance';
import appIcon from '../assets/icon-192.png';
import { useI18n } from '../i18n';

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
const layerOperationError = ref('');
const pendingRenameLayerPath = ref('');
const renameLayerError = ref('');
const renamingLayer = ref(false);
const deletingLayerMap = ref<Record<string, boolean>>({});
const pendingDeleteLayerPath = ref('');
const { locale, setLocale, supportedLocales, t } = useI18n();
const { shelves, loading: shelvesLoading, loaded: shelvesLoaded, error: shelvesError, selectedShelfID, fetchShelves, selectShelf } = useShelvesStore();
const { readOnly, fetchServerMode } = useServerMode();
const localeLabelKeyMap: Record<(typeof supportedLocales)[number], 'language.en' | 'language.zhHant'> = {
  en: 'language.en',
  'zh-Hant': 'language.zhHant'
};

const currentLayer = computed(() => {
  const q = route.query.layers;
  return typeof q === 'string' && q.length > 0 ? q : undefined;
});

const layerTree = computed(() => buildLayerTreeNodes(layers.value));
const canSubmitCreateLayer = computed(() => normalizeLayerPath(newLayerPath.value).length > 0);
const isDeletingPendingLayer = computed(
  () => pendingDeleteLayerPath.value.length > 0 && (deletingLayerMap.value[pendingDeleteLayerPath.value] ?? false)
);
const pendingRenameLayerName = computed(() => {
  const segments = pendingRenameLayerPath.value.split('/').filter((segment) => segment.length > 0);
  return segments[segments.length - 1] ?? '';
});
const isRenamingPendingLayer = computed(() => pendingRenameLayerPath.value.length > 0 && renamingLayer.value);
const hasActiveShelf = computed(() => shelvesLoaded.value && selectedShelfID.value.length > 0);
const isSettingsRoute = computed(() => route.name === 'settings');
const canShowRouteContent = computed(() => isSettingsRoute.value || hasActiveShelf.value);
const shelfUnavailableMessage = computed(() =>
  shelvesLoading.value ? t('layout.shelf.loading') : t('layout.shelf.unavailableDescription')
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
  layerOperationError.value = '';
  goToLayer(normalizeLayerSelectionPath(path));
}

function onLocaleChange(event: Event): void {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }

  if (supportedLocales.includes(target.value as (typeof supportedLocales)[number])) {
    setLocale(target.value as (typeof supportedLocales)[number]);
  }
}

async function onShelfChange(event: Event): Promise<void> {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }

  const nextShelfID = target.value.trim();
  if (!nextShelfID || nextShelfID === selectedShelfID.value) {
    return;
  }

  selectShelf(nextShelfID);
  deleteLayerError.value = '';
  layerOperationError.value = '';
  moveBookError.value = '';
  createLayerError.value = '';
  createLayerSuccess.value = '';

  await Promise.all([fetchLayers(), fetchBooks()]);
  await router.push({ path: '/books', query: { page: '1' } });
}

function toggleCreateLayerForm(): void {
  if (readOnly.value) {
    return;
  }
  showCreateLayerForm.value = !showCreateLayerForm.value;
  createLayerError.value = '';
  createLayerSuccess.value = '';

  if (!showCreateLayerForm.value) {
    newLayerPath.value = '';
  }
}

async function onSubmitCreateLayer(): Promise<void> {
  if (readOnly.value) {
    createLayerError.value = t('layout.readOnly.writeDisabled');
    return;
  }
  const normalized = normalizeLayerPath(newLayerPath.value);
  if (!normalized) {
    createLayerError.value = t('layout.layerErrors.emptyPath');
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
    createLayerSuccess.value = t('layout.createLayer.created');
    newLayerPath.value = '';
    showCreateLayerForm.value = false;
  } catch (err) {
    const message = err instanceof Error ? err.message : t('layout.layerErrors.createFailed');

    if (message === 'Layer path cannot be empty') {
      createLayerError.value = t('layout.layerErrors.emptyPath');
    } else if (message === 'Failed to create layer') {
      createLayerError.value = t('layout.layerErrors.createFailed');
    } else {
      createLayerError.value = message || t('layout.layerErrors.createFailed');
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
  if (readOnly.value) {
    moveBookError.value = t('layout.readOnly.writeDisabled');
    return;
  }
  moveBookError.value = '';
  layerOperationError.value = '';

  const currentBook = books.value.find((item) => item.id === payload.bookId);
  if (!currentBook) {
    moveBookError.value = t('layout.moveBookErrors.notFound');
    return;
  }

  const currentLayerPath = getLayerPath(currentBook);
  if (currentLayerPath === payload.targetLayer) {
    return;
  }

  try {
    await getBookshelfProvider().updateBookLayer(payload.bookId, payload.targetLayer);
    await fetchBooks();
  } catch (err) {
    moveBookError.value = err instanceof Error ? err.message : t('layout.moveBookErrors.failed');
  }
}

function requestRenameLayer(path: string): void {
  if (readOnly.value) {
    layerOperationError.value = t('layout.readOnly.writeDisabled');
    return;
  }

  pendingRenameLayerPath.value = path;
  renameLayerError.value = '';
  layerOperationError.value = '';
}

function cancelPendingRenameLayer(): void {
  if (renamingLayer.value) {
    return;
  }

  pendingRenameLayerPath.value = '';
  renameLayerError.value = '';
}

async function confirmRenameLayer(nextName: string): Promise<void> {
  const path = pendingRenameLayerPath.value;
  if (!path || renamingLayer.value) {
    return;
  }

  if (!nextName || nextName === pendingRenameLayerName.value) {
    renameLayerError.value = t('layout.renameLayer.invalid');
    return;
  }

  renamingLayer.value = true;
  renameLayerError.value = '';
  layerOperationError.value = '';

  try {
    await renameLayer(path, nextName);
    await Promise.all([fetchLayers(), fetchBooks()]);

    if (currentLayer.value === path || currentLayer.value?.startsWith(`${path}/`)) {
      const parent = path.split('/').filter((segment) => segment.length > 0).slice(0, -1);
      const renamedPath = [...parent, nextName].join('/');
      goToLayer(currentLayer.value === path ? renamedPath : `${renamedPath}${currentLayer.value.slice(path.length)}`);
    }

    pendingRenameLayerPath.value = '';
  } catch (err) {
    const message = err instanceof Error ? err.message : '';
    if (message === 'Invalid layer name') {
      renameLayerError.value = t('layout.renameLayer.invalid');
    } else {
      renameLayerError.value = message || t('layout.renameLayer.failed');
    }
  } finally {
    renamingLayer.value = false;
  }
}

async function onMoveLayer(payload: { layerPath: string; targetLayer: string }): Promise<void> {
  if (readOnly.value) {
    layerOperationError.value = t('layout.readOnly.writeDisabled');
    return;
  }
  layerOperationError.value = '';

  try {
    await moveLayer(payload.layerPath, payload.targetLayer);
    await Promise.all([fetchLayers(), fetchBooks()]);

    if (currentLayer.value === payload.layerPath || currentLayer.value?.startsWith(`${payload.layerPath}/`)) {
      const layerSegments = payload.layerPath.split('/').filter((segment) => segment.length > 0);
      const layerName = layerSegments[layerSegments.length - 1];
      if (layerName) {
        const targetSegments = payload.targetLayer === '/' ? [] : payload.targetLayer.split('/').filter(Boolean);
        const movedPath = [...targetSegments, layerName].join('/');
        goToLayer(currentLayer.value === payload.layerPath ? movedPath : `${movedPath}${currentLayer.value.slice(payload.layerPath.length)}`);
      }
    }
  } catch (err) {
    const message = err instanceof Error ? err.message : '';
    layerOperationError.value = message || t('layout.moveLayer.failed');
  }
}

function requestDeleteLayer(path: string): void {
  if (readOnly.value) {
    deleteLayerError.value = t('layout.readOnly.writeDisabled');
    return;
  }
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
      deleteLayerError.value = t('layout.deleteLayer.notEmpty');
    } else if (message) {
      deleteLayerError.value = message;
    } else {
      deleteLayerError.value = t('layout.deleteLayer.failed');
    }
  } finally {
    const { [path]: _deleted, ...rest } = deletingLayerMap.value;
    deletingLayerMap.value = rest;
  }
}


onMounted(async () => {
  await fetchServerMode();
  if (!shelvesLoaded.value) {
    await fetchShelves();
  }

  if (!hasActiveShelf.value) {
    return;
  }

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

.read-only-banner {
  background: #fef3c7;
  border-bottom: 1px solid #f59e0b;
  color: #92400e;
  font-size: 13px;
  font-weight: 700;
  padding: 8px 24px;
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

.topbar-controls {
  display: inline-flex;
  gap: 10px;
}

.language-select {
  align-items: center;
  display: inline-flex;
  gap: 8px;
}

.language-select span {
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.language-select-control {
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-size: 13px;
  min-height: 32px;
  padding: 0 8px;
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

.no-shelf-panel {
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 12px;
  color: var(--text);
  display: grid;
  gap: 10px;
  margin: 32px auto;
  max-width: 560px;
  padding: 24px;
}

.no-shelf-panel h2,
.no-shelf-panel p {
  margin: 0;
}

.no-shelf-panel .button {
  justify-self: start;
}

.sidebar-shelf-label {
  align-items: center;
  display: flex;
  gap: 8px;
  padding: 4px 8px;
}

.sidebar-shelf-select {
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  flex: 1;
  font-size: 13px;
  min-height: 30px;
  min-width: 0;
  padding: 0 6px;
}

</style>
