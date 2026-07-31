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
    <CreateLayerModal
      :open="showCreateLayerModal"
      :parent-options="createLayerParentOptions"
      :default-parent="createLayerDefaultParent"
      :busy="creatingLayer"
      :error="createLayerError"
      @cancel="closeCreateLayerModal"
      @submit="onSubmitCreateLayer"
    />

    <div
      v-if="isNarrowViewport && drawerOpen"
      class="sidebar-backdrop"
      aria-hidden="true"
      @click="drawerOpen = false"
    ></div>

    <SplitterGroup
      direction="horizontal"
      as="div"
      auto-save-id="plainshelf-sidebar"
      :keyboard-resize-by="16"
      class="layout-splitter"
    >
    <SplitterPanel
      ref="sidebarPanelRef"
      as="aside"
      class="sidebar"
      :class="{ 'sidebar-drawer-open': isNarrowViewport && drawerOpen, 'sidebar-rail': isRailSidebar }"
      size-unit="px"
      :default-size="240"
      :min-size="isRailSidebar ? RAIL_SIDEBAR_WIDTH : 200"
      :max-size="isRailSidebar ? RAIL_SIDEBAR_WIDTH : 300"
    >
      <button
        class="collapse-btn"
        type="button"
        :aria-label="isRailSidebar ? t('layout.expandSidebar') : t('layout.collapseSidebar')"
        @click="toggleSidebarMode"
      >
        {{ isRailSidebar ? '→' : '←' }}
      </button>

      <TooltipProvider v-if="showRailNav" :delay-duration="300">
        <nav class="sidebar-rail-nav" :aria-label="t('layout.railNavLabel')">
          <template v-if="hasActiveShelf">
            <TooltipRoot>
              <TooltipTrigger as-child>
                <RouterLink
                  to="/dashboard"
                  class="sidebar-nav-item sidebar-rail-item"
                  exact-active-class="active"
                  :aria-label="t('layout.dashboard')"
                >
                  <SidebarNavIcon name="dashboard" />
                </RouterLink>
              </TooltipTrigger>
              <TooltipPortal>
                <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
                  {{ t('layout.dashboard') }}
                </TooltipContent>
              </TooltipPortal>
            </TooltipRoot>
            <TooltipRoot>
              <TooltipTrigger as-child>
                <RouterLink
                  to="/read-history"
                  class="sidebar-nav-item sidebar-rail-item"
                  exact-active-class="active"
                  :aria-label="t('layout.recentlyRead')"
                >
                  <SidebarNavIcon name="recently-read" />
                </RouterLink>
              </TooltipTrigger>
              <TooltipPortal>
                <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
                  {{ t('layout.recentlyRead') }}
                </TooltipContent>
              </TooltipPortal>
            </TooltipRoot>
            <TooltipRoot>
              <TooltipTrigger as-child>
                <RouterLink
                  to="/trash"
                  class="sidebar-nav-item sidebar-rail-item"
                  exact-active-class="active"
                  :aria-label="t('layout.trash')"
                >
                  <SidebarNavIcon name="trash" />
                </RouterLink>
              </TooltipTrigger>
              <TooltipPortal>
                <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
                  {{ t('layout.trash') }}
                </TooltipContent>
              </TooltipPortal>
            </TooltipRoot>
          </template>
          <TooltipRoot v-if="isMobileEnv">
            <TooltipTrigger as-child>
              <RouterLink
                to="/downloads"
                class="sidebar-nav-item sidebar-rail-item"
                exact-active-class="active"
                :aria-label="t('layout.downloads')"
              >
                <SidebarNavIcon name="downloads" />
              </RouterLink>
            </TooltipTrigger>
            <TooltipPortal>
              <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
                {{ t('layout.downloads') }}
              </TooltipContent>
            </TooltipPortal>
          </TooltipRoot>
          <TooltipRoot v-if="hasActiveShelf">
            <TooltipTrigger as-child>
              <RouterLink
                to="/admin/logs"
                class="sidebar-nav-item sidebar-rail-item"
                exact-active-class="active"
                :aria-label="t('layout.adminLogs')"
              >
                <SidebarNavIcon name="logs" />
              </RouterLink>
            </TooltipTrigger>
            <TooltipPortal>
              <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
                {{ t('layout.adminLogs') }}
              </TooltipContent>
            </TooltipPortal>
          </TooltipRoot>
          <TooltipRoot>
            <TooltipTrigger as-child>
              <RouterLink
                to="/settings"
                class="sidebar-nav-item sidebar-rail-item"
                exact-active-class="active"
                :aria-label="t('layout.settings')"
              >
                <SidebarNavIcon name="settings" />
              </RouterLink>
            </TooltipTrigger>
            <TooltipPortal>
              <TooltipContent class="reka-tooltip" side="right" :side-offset="8">
                {{ t('layout.settings') }}
              </TooltipContent>
            </TooltipPortal>
          </TooltipRoot>
        </nav>
      </TooltipProvider>

      <div v-if="!isRailSidebar || isNarrowViewport" class="sidebar-inner">
        <section class="sidebar-section" :aria-label="t('layout.shelf.label')">
          <label class="sidebar-shelf-label">
            <span class="sidebar-section-title">{{ t('layout.shelf.label') }}</span>
            <SelectRoot
              :model-value="selectedShelfID"
              :disabled="shelvesLoading || shelves.length === 0"
              @update:model-value="onShelfSelect"
            >
              <SelectTrigger class="button sidebar-shelf-select">
                <SelectValue :placeholder="shelfSelectPlaceholder" />
              </SelectTrigger>
              <SelectPortal>
                <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                  <SelectViewport>
                    <SelectItem v-for="shelf in shelves" :key="shelf.id" class="reka-menu-item" :value="shelf.id">
                      <SelectItemText>{{ shelf.name }}</SelectItemText>
                    </SelectItem>
                  </SelectViewport>
                </SelectContent>
              </SelectPortal>
            </SelectRoot>
          </label>
          <p v-if="shelvesError" class="sidebar-error" role="alert">{{ shelvesError }}</p>
        </section>

        <template v-if="hasActiveShelf">
          <div class="sidebar-nav-divider" role="presentation"></div>
          <section class="sidebar-section" :aria-label="t('layout.sections.layers')">
            <div class="sidebar-header-row sidebar-foldable-header">
              <button
                type="button"
                class="sidebar-section-toggle"
                :aria-label="t('layout.sectionToggleLabels.layers')"
                :aria-expanded="!collapsedSidebarSections.layers"
                aria-controls="sidebar-section-layers"
                @click="toggleSidebarSection('layers')"
              >
                <span class="sidebar-section-title" aria-hidden="true">{{ t('layout.sections.layers') }}</span>
                <span class="sidebar-section-toggle-icon" aria-hidden="true">{{ collapsedSidebarSections.layers ? '▸' : '▾' }}</span>
              </button>
              <button
                type="button"
                class="create-layer-toggle"
                aria-haspopup="dialog"
                :disabled="readOnly || creatingLayer || layersLoading || layersError.length > 0"
                @click="openCreateLayerModal"
              >
                {{ t('layout.createLayer.add') }}
              </button>
            </div>

            <div v-show="!collapsedSidebarSections.layers" id="sidebar-section-layers" class="sidebar-foldable-content">
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
                :can-open-layer-folder="canOpenLayerFolder"
                @select="onSelectLayer"
                @move-book="onMoveBook"
                @delete-layer="requestDeleteLayer"
                @rename-layer="requestRenameLayer"
                @open-layer-folder="onOpenLayerFolder"
                @move-layer="onMoveLayer"
              />
              <p v-if="moveBookError" class="sidebar-error" role="alert">
                {{ moveBookError }}
              </p>
              <p v-if="deleteLayerError && !pendingDeleteLayerPath" class="sidebar-error sidebar-error-pre" role="alert">
                {{ deleteLayerError }}
              </p>
              <p v-if="layerOperationError" class="sidebar-error sidebar-error-pre" role="alert">
                {{ layerOperationError }}
              </p>
            </div>
          </section>

          <div class="sidebar-nav-divider" role="presentation"></div>

          <section class="sidebar-section" :aria-label="t('layout.sections.reading')">
            <button
              type="button"
              class="sidebar-section-toggle"
              :aria-label="t('layout.sectionToggleLabels.reading')"
              :aria-expanded="!collapsedSidebarSections.reading"
              aria-controls="sidebar-section-reading"
              @click="toggleSidebarSection('reading')"
            >
              <span class="sidebar-section-title" aria-hidden="true">{{ t('layout.sections.reading') }}</span>
              <span class="sidebar-section-toggle-icon" aria-hidden="true">{{ collapsedSidebarSections.reading ? '▸' : '▾' }}</span>
            </button>
            <nav
              v-show="!collapsedSidebarSections.reading"
              id="sidebar-section-reading"
              class="sidebar-nav-list sidebar-foldable-content"
              :aria-label="t('layout.sections.reading')"
            >
              <RouterLink
                to="/dashboard"
                class="sidebar-nav-item"
                exact-active-class="active"
              >
                <SidebarNavIcon name="dashboard" />
                <span>{{ t('layout.dashboard') }}</span>
              </RouterLink>
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
            <button
              type="button"
              class="sidebar-section-toggle"
              :aria-label="t('layout.sectionToggleLabels.maintenance')"
              :aria-expanded="!collapsedSidebarSections.maintenance"
              aria-controls="sidebar-section-maintenance"
              @click="toggleSidebarSection('maintenance')"
            >
              <span class="sidebar-section-title" aria-hidden="true">{{ t('layout.sections.maintenance') }}</span>
              <span class="sidebar-section-toggle-icon" aria-hidden="true">{{ collapsedSidebarSections.maintenance ? '▸' : '▾' }}</span>
            </button>
            <nav
              v-show="!collapsedSidebarSections.maintenance"
              id="sidebar-section-maintenance"
              class="sidebar-nav-list sidebar-foldable-content"
              :aria-label="t('layout.sections.maintenance')"
            >
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

        <template v-if="isMobileEnv">
          <div class="sidebar-nav-divider" role="presentation"></div>
          <section class="sidebar-section" :aria-label="t('layout.downloads')">
            <nav class="sidebar-nav-list" :aria-label="t('layout.downloads')">
              <RouterLink to="/downloads" class="sidebar-nav-item" exact-active-class="active">
                <SidebarNavIcon name="downloads" />
                <span>{{ t('layout.downloads') }}</span>
              </RouterLink>
            </nav>
          </section>
        </template>

        <div class="sidebar-nav-divider" role="presentation"></div>

        <section class="sidebar-section" :aria-label="t('layout.sections.admin')">
          <button
            type="button"
            class="sidebar-section-toggle"
            :aria-label="t('layout.sectionToggleLabels.admin')"
            :aria-expanded="!collapsedSidebarSections.admin"
            aria-controls="sidebar-section-admin"
            @click="toggleSidebarSection('admin')"
          >
            <span class="sidebar-section-title" aria-hidden="true">{{ t('layout.sections.admin') }}</span>
            <span class="sidebar-section-toggle-icon" aria-hidden="true">{{ collapsedSidebarSections.admin ? '▸' : '▾' }}</span>
          </button>
          <nav
            v-show="!collapsedSidebarSections.admin"
            id="sidebar-section-admin"
            class="sidebar-nav-list sidebar-foldable-content"
            :aria-label="t('layout.sections.admin')"
          >
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
    </SplitterPanel>

    <SplitterResizeHandle
      as="div"
      class="reka-resize-handle"
      :class="{ 'rail-hidden': isRailSidebar }"
      :disabled="isRailSidebar"
      :hit-area-margins="SIDEBAR_RESIZE_HIT_AREA_MARGINS"
    />

    <SplitterPanel as="main" class="main-content">
      <!-- SplitterPanel forces inline overflow:hidden, so scrolling lives on
           this inner wrapper (same pattern as .sidebar-inner). -->
      <div class="main-scroll">
      <div v-if="readOnly" class="read-only-banner" role="status">
        {{ t('layout.readOnly.banner') }}
      </div>
      <header class="topbar">
        <div class="topbar-left">
          <button
            v-if="isNarrowViewport"
            class="menu-btn"
            type="button"
            :aria-label="t(drawerOpen ? 'layout.closeMenu' : 'layout.openMenu')"
            :aria-expanded="drawerOpen"
            @click="drawerOpen = !drawerOpen"
          >
            ☰
          </button>
          <h1 class="brand">
            <img class="brand-icon" :src="appIcon" alt="" aria-hidden="true">
            <span>{{ t('app.name') }}</span>
          </h1>
        </div>
        <div class="topbar-controls">
          <label class="language-select">
            <span>{{ t('language.label') }}</span>
            <SelectRoot :model-value="locale" @update:model-value="onLocaleSelect">
              <SelectTrigger class="button language-select-control">
                <SelectValue />
              </SelectTrigger>
              <SelectPortal>
                <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
                  <SelectViewport>
                    <SelectItem v-for="lang in supportedLocales" :key="lang" class="reka-menu-item" :value="lang">
                      <SelectItemText>{{ t(localeLabelKeyMap[lang]) }}</SelectItemText>
                    </SelectItem>
                  </SelectViewport>
                </SelectContent>
              </SelectPortal>
            </SelectRoot>
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
      </div>
    </SplitterPanel>
    </SplitterGroup>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  SplitterGroup,
  SplitterPanel,
  SplitterResizeHandle,
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  type AcceptableValue
} from 'reka-ui';
import CreateLayerModal, { type CreateLayerParentOption } from '@/components/CreateLayerModal.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import LayerTree from '@/components/LayerTree.vue';
import RenameLayerModal from '@/components/RenameLayerModal.vue';
import SidebarNavIcon from '@/components/SidebarNavIcon.vue';
import { getBookshelfProvider, isMobileRuntime } from '@/providers';
import { createLayer, deleteLayer, moveLayer, renameLayer } from '@/api/layers';
import { useBookStore } from '@/composables/useBookStore';
import { useLayerStore } from '@/composables/useLayerStore';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useServerMode } from '@/composables/useServerMode';
import {
  RAIL_SIDEBAR_WIDTH,
  SIDEBAR_RESIZE_HIT_AREA_MARGINS,
  useSidebarLayout
} from '@/composables/useSidebarLayout';
import { buildLayerTreeNodes, flattenLayerTreePaths, getLayerPath, normalizeLayerPath } from '@/utils/layers';
import { MAINTENANCE_NAV_ITEMS } from '@/utils/maintenance';
import appIcon from '@/assets/icon-192.png';
import { useI18n } from '@/i18n';

const {
  sidebarPanelRef,
  isRailSidebar,
  showRailNav,
  isNarrowViewport,
  drawerOpen,
  collapsedSidebarSections,
  toggleSidebarMode,
  toggleSidebarSection
} = useSidebarLayout();

const route = useRoute();
const router = useRouter();

// Matches the isMobileEnv pattern in SettingsPage.vue; runtime does not
// change during a session, but a computed keeps it consistent with the
// other environment checks used in the template.
const isMobileEnv = computed(() => isMobileRuntime());
const { books, loading, fetchBooks } = useBookStore();
const { layers, loading: layersLoading, error: layersError, loaded: layersLoaded, fetchLayers } = useLayerStore();
const moveBookError = ref('');
const showCreateLayerModal = ref(false);
const creatingLayer = ref(false);
const createLayerError = ref('');
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
const canOpenLayerFolder = computed(() => Boolean(getBookshelfProvider().openDesktopLayerFolder));
const createLayerParentOptions = computed<CreateLayerParentOption[]>(() => [
  { value: '/', label: t('layout.createLayer.rootOption'), depth: 0 },
  ...flattenLayerTreePaths(layerTree.value).map((option) => ({
    value: option.path,
    label: option.path,
    depth: option.depth + 1
  }))
]);
const createLayerDefaultParent = computed(() => normalizeLayerPath(currentLayer.value ?? '') || '/');
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
const shelfSelectPlaceholder = computed(() => {
  if (shelvesLoading.value) {
    return t('layout.shelf.loading');
  }
  if (shelves.value.length === 0) {
    return t('layout.shelf.empty');
  }
  return '';
});

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

function onLocaleSelect(value: AcceptableValue): void {
  if (typeof value !== 'string') {
    return;
  }

  if (supportedLocales.includes(value as (typeof supportedLocales)[number])) {
    setLocale(value as (typeof supportedLocales)[number]);
  }
}

async function onShelfSelect(value: AcceptableValue): Promise<void> {
  if (typeof value !== 'string') {
    return;
  }

  const nextShelfID = value.trim();
  if (!nextShelfID || nextShelfID === selectedShelfID.value) {
    return;
  }

  selectShelf(nextShelfID);
  deleteLayerError.value = '';
  layerOperationError.value = '';
  moveBookError.value = '';
  createLayerError.value = '';

  await Promise.all([fetchLayers(), fetchBooks()]);
  await router.push({ path: '/books', query: { page: '1' } });
}

function openCreateLayerModal(): void {
  if (readOnly.value) {
    return;
  }

  createLayerError.value = '';
  showCreateLayerModal.value = true;
}

function closeCreateLayerModal(): void {
  if (creatingLayer.value) {
    return;
  }

  showCreateLayerModal.value = false;
  createLayerError.value = '';
}

async function onSubmitCreateLayer(payload: { parentPath: string; name: string }): Promise<void> {
  if (readOnly.value) {
    createLayerError.value = t('layout.readOnly.writeDisabled');
    return;
  }

  const name = payload.name.trim();
  if (!name || name.includes('/')) {
    createLayerError.value = t('layout.createLayer.invalidName');
    return;
  }

  // normalizeLayerPath drops empty segments, so a '/' parent joins cleanly.
  const normalized = normalizeLayerPath(`${payload.parentPath}/${name}`);
  if (!normalized) {
    createLayerError.value = t('layout.layerErrors.emptyPath');
    return;
  }

  creatingLayer.value = true;
  createLayerError.value = '';

  try {
    await createLayer(normalized);
    await fetchLayers();

    showCreateLayerModal.value = false;
    goToLayer(normalized);
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

async function onOpenLayerFolder(path: string): Promise<void> {
  layerOperationError.value = '';
  const openDesktopLayerFolder = getBookshelfProvider().openDesktopLayerFolder;
  if (!openDesktopLayerFolder) {
    return;
  }

  try {
    await openDesktopLayerFolder(path);
  } catch (err) {
    const message = err instanceof Error ? err.message : '';
    layerOperationError.value = message || t('layout.openLayerFolder.failed');
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
  height: calc(100vh / var(--app-zoom, 1));
  width: calc(100vw / var(--app-zoom, 1));
  overflow: hidden;
}

.layout-splitter {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

/* ── Sidebar ── */
.sidebar {
  border-right: 1px solid var(--border);
  background: linear-gradient(180deg, #e9edf2 0%, #e3e8ef 100%);
  backdrop-filter: blur(8px);
  display: flex;
  flex-direction: column;
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

.sidebar-rail-nav {
  align-items: center;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 4px 8px;
}

.sidebar-rail-item {
  height: 32px;
  justify-content: center;
  padding: 0;
  width: 32px;
}

.sidebar-rail-item :deep(.sidebar-nav-icon) {
  margin-right: 0;
}

.reka-resize-handle.rail-hidden {
  display: none;
}

.sidebar-inner {
  flex: 1;
  min-height: 0;
  min-width: 176px;
  overflow-y: auto;
  padding: 8px;
}

.sidebar-header-row {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.sidebar-foldable-header {
  gap: 8px;
}

.sidebar-section-toggle {
  align-items: center;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: #4f5d72;
  cursor: pointer;
  display: flex;
  flex: 1;
  gap: 4px;
  justify-content: space-between;
  min-height: 28px;
  min-width: 0;
  padding: 0 4px 0 0;
  text-align: left;
}

.sidebar-section-toggle:hover {
  background: #eef3f9;
  border-color: #d4deea;
}

.sidebar-section-toggle:focus-visible {
  outline: 2px solid #2563eb;
  outline-offset: 2px;
}

.sidebar-section-toggle .sidebar-section-title {
  flex: 1;
}

.sidebar-section-toggle-icon {
  color: #64748b;
  flex: 0 0 auto;
  font-size: 12px;
  line-height: 1;
}

.sidebar-foldable-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
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

.sidebar-error {
  color: #b91c1c;
  font-size: 12px;
  line-height: 1.4;
  margin: 8px 8px 0;
}

.sidebar-error-pre {
  white-space: pre-line;
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
  max-width: none;
  background: white;
  min-width: 0;
}

.main-scroll {
  height: 100%;
  overflow-y: auto;
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

.topbar-left {
  align-items: center;
  display: inline-flex;
  gap: 10px;
  min-width: 0;
}

.menu-btn {
  align-items: center;
  background: #f6f9fc;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: #3e4e66;
  cursor: pointer;
  display: flex;
  font-size: 16px;
  height: 34px;
  justify-content: center;
  width: 38px;
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

/* ── Narrow viewport (mobile): sidebar becomes an off-canvas drawer ── */

.sidebar-backdrop {
  background: rgba(15, 23, 42, 0.45);
  inset: 0;
  position: fixed;
  z-index: 40;
}

/* Keep in sync with NARROW_VIEWPORT_QUERY in composables/useViewport.ts. */
@media (max-width: 768px) {
  /* position:fixed takes the sidebar out of the splitter's flex flow, so the
     main panel gets the full width and the splitter's inline flex-basis on
     the sidebar stops mattering. Drag/collapse make no sense here. */
  .sidebar {
    bottom: 0;
    box-shadow: 4px 0 24px rgba(15, 23, 42, 0.25);
    left: 0;
    position: fixed;
    top: 0;
    transform: translateX(-105%);
    transition: transform 0.2s ease;
    width: min(300px, calc(100vw / var(--app-zoom, 1) - 48px));
    z-index: 41;
  }

  .sidebar.sidebar-drawer-open {
    transform: translateX(0);
  }

  .collapse-btn,
  .reka-resize-handle {
    display: none;
  }

  .topbar {
    padding: 10px 12px;
  }

  .language-select > span {
    display: none;
  }
}
</style>
