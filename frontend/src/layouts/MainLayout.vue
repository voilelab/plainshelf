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
        <section class="sidebar-section" :aria-label="t('layout.sections.layers')">
          <div class="sidebar-header-row">
            <div class="sidebar-section-title">{{ t('layout.sections.layers') }}</div>
            <button
              type="button"
              class="create-layer-toggle"
              :disabled="creatingLayer"
              @click="toggleCreateLayerForm"
            >
              {{ showCreateLayerForm ? t('layout.createLayer.cancel') : t('layout.createLayer.add') }}
            </button>
          </div>

          <form v-if="showCreateLayerForm" class="create-layer-form" @submit.prevent="onSubmitCreateLayer">
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
                :disabled="creatingLayer || !canSubmitCreateLayer"
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
      </div>
    </aside>

    <main class="main-content">
      <header class="topbar">
        <h1 class="brand">
          <img class="brand-icon" :src="appIcon" alt="" aria-hidden="true">
          <span>{{ t('app.name') }}</span>
        </h1>
        <label class="language-select">
          <span>{{ t('language.label') }}</span>
          <select class="language-select-control" :value="locale" @change="onLocaleChange">
            <option v-for="lang in supportedLocales" :key="lang" :value="lang">
              {{ t(localeLabelKeyMap[lang]) }}
            </option>
          </select>
        </label>
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
import SidebarNavIcon from '../components/SidebarNavIcon.vue';
import { updateBookLayer } from '../api/books';
import { createLayer, deleteLayer } from '../api/layers';
import { useBookStore } from '../composables/useBookStore';
import { useLayerStore } from '../composables/useLayerStore';
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
const deletingLayerMap = ref<Record<string, boolean>>({});
const pendingDeleteLayerPath = ref('');
const { locale, setLocale, supportedLocales, t } = useI18n();
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

function onLocaleChange(event: Event): void {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }

  if (supportedLocales.includes(target.value as (typeof supportedLocales)[number])) {
    setLocale(target.value as (typeof supportedLocales)[number]);
  }
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
  moveBookError.value = '';

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
    await updateBookLayer(payload.bookId, payload.targetLayer);
    await fetchBooks();
  } catch (err) {
    moveBookError.value = err instanceof Error ? err.message : t('layout.moveBookErrors.failed');
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
</style>
