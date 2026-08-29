<template>
  <div class="layout-root">
    <DeleteModal
      :open="pendingDeleteFolderPath.length > 0"
      :title="t('layout.deleteFolder.title')"
      :item-name="pendingDeleteFolderPath"
      :description="t('layout.deleteFolder.description')"
      :busy="isDeletingPendingFolder"
      :error="deleteFolderError"
      @cancel="cancelPendingDeleteFolder"
      @confirm="confirmDeleteFolder"
    />
    <RenameFolderModal
      :open="pendingRenameFolderPath.length > 0"
      :current-name="pendingRenameFolderName"
      :busy="isRenamingPendingFolder"
      :error="renameFolderError"
      @cancel="cancelPendingRenameFolder"
      @submit="confirmRenameFolder"
    />
    <CreateFolderModal
      :open="showCreateFolderModal"
      :busy="creatingFolder"
      :error="createFolderError"
      @cancel="closeCreateFolderModal"
      @submit="onSubmitCreateFolder"
    />
    <TransferFolderModal
      :open="transferFolderTarget.length > 0"
      :folder-name="transferFolderName"
      :busy="transferFolderRunning"
      :started="transferFolderStarted"
      :finished="transferFolderFinished"
      :status="transferFolderStatus"
      :percentage="transferFolderPercentage"
      :chain="transferFolderChain"
      :error="transferFolderError"
      @close="cancelTransferFolder"
      @submit="submitTransferFolder"
    />
    <BookBatchProgressModal />

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
                  to="/home"
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
            <TooltipRoot v-if="libraryEditingAvailable">
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
          <TooltipRoot v-if="hasDownloadsStore">
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
          <TooltipRoot v-if="hasActiveShelf && serverAdminAvailable">
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
              :model-value="shelfPicker.value.value"
              :disabled="shelfPicker.disabled.value"
              @update:model-value="onShelfSelect"
            >
              <SelectTrigger class="button sidebar-shelf-select">
                <SelectValue :placeholder="shelfPicker.placeholder.value" />
              </SelectTrigger>
              <SelectPortal>
                <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                  <SelectViewport>
                    <SelectItem v-for="item in shelfPicker.items.value" :key="item.id" class="reka-menu-item" :value="item.id">
                      <SelectItemText>{{ item.name }}</SelectItemText>
                      <!-- Sibling of SelectItemText, not inside it: the trigger
                           echoes the item text, and a badge in there would read
                           as part of the shelf's name. -->
                      <span v-if="item.typeLabel" class="sidebar-shelf-type">{{ item.typeLabel }}</span>
                    </SelectItem>
                  </SelectViewport>
                </SelectContent>
              </SelectPortal>
            </SelectRoot>
          </label>
          <RouterLink v-if="shelfManageTo" :to="shelfManageTo" class="sidebar-shelf-manage">
            {{ t('layout.shelf.manage') }}
          </RouterLink>
          <p v-if="shelfPicker.error.value" class="sidebar-error" role="alert">{{ shelfPicker.error.value }}</p>
        </section>

        <template v-if="hasActiveShelf">
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
                to="/home"
                class="sidebar-nav-item"
                exact-active-class="active"
              >
                <SidebarNavIcon name="dashboard" />
                <span>{{ t('layout.dashboard') }}</span>
              </RouterLink>
              <RouterLink
                v-if="!isMobileShell"
                to="/read-history"
                class="sidebar-nav-item"
                exact-active-class="active"
              >
                <SidebarNavIcon name="recently-read" />
                <span>{{ t('layout.recentlyRead') }}</span>
              </RouterLink>
              <RouterLink
                v-if="libraryEditingAvailable"
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

          <section class="sidebar-section" :aria-label="t('layout.sections.folders')">
            <div class="sidebar-header-row sidebar-foldable-header">
              <button
                type="button"
                class="sidebar-section-toggle"
                :aria-label="t('layout.sectionToggleLabels.folders')"
                :aria-expanded="!collapsedSidebarSections.folders"
                aria-controls="sidebar-section-folders"
                @click="toggleSidebarSection('folders')"
              >
                <span class="sidebar-section-title" aria-hidden="true">{{ t('layout.sections.folders') }}</span>
                <span class="sidebar-section-toggle-icon" aria-hidden="true">{{ collapsedSidebarSections.folders ? '▸' : '▾' }}</span>
              </button>
            </div>

            <div v-show="!collapsedSidebarSections.folders" id="sidebar-section-folders" class="sidebar-foldable-content">
              <div v-if="foldersLoading" class="sidebar-status">{{ t('layout.createFolder.loadingFolders') }}</div>
              <div v-else-if="foldersError" class="sidebar-status sidebar-error sidebar-folder-error" role="alert">
                <p>{{ foldersError }}</p>
                <button type="button" class="button" @click="fetchFolders">{{ t('common.retry') }}</button>
              </div>
              <FolderTree
                v-else
                :nodes="folderTree"
                :selected="currentFolder"
                :deleting-map="deletingFolderMap"
                :read-only="readOnly"
                :can-open-folder="canOpenFolder"
                :can-transfer-folder="canTransferFolder"
                @select="onSelectFolder"
                @create-folder="openCreateFolderModal"
                @move-book="onMoveBook"
                @delete-folder="requestDeleteFolder"
                @rename-folder="requestRenameFolder"
                @open-folder="onOpenFolder"
                @transfer-folder="requestTransferFolder"
                @move-folder="onMoveFolder"
              />
              <p v-if="moveBookError" class="sidebar-error" role="alert">
                {{ moveBookError }}
              </p>
              <p v-if="deleteFolderError && !pendingDeleteFolderPath" class="sidebar-error sidebar-error-pre" role="alert">
                {{ deleteFolderError }}
              </p>
              <p v-if="folderOperationError" class="sidebar-error sidebar-error-pre" role="alert">
                {{ folderOperationError }}
              </p>
            </div>
          </section>

          <div v-if="libraryEditingAvailable" class="sidebar-nav-divider" role="presentation"></div>

          <section
            v-if="libraryEditingAvailable"
            class="sidebar-section"
            :aria-label="t('layout.sections.maintenance')"
          >
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

        <!-- Downloads and the admin section (Logs + Settings) are reached from
             the bottom tab bar on the mobile shell, so the drawer drops them
             there and keeps only shelf switching and the folder tree. -->
        <template v-if="hasDownloadsStore && !isMobileShell">
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

        <div v-if="!isMobileShell" class="sidebar-nav-divider" role="presentation"></div>

        <section
          v-if="!isMobileShell"
          class="sidebar-section"
          :aria-label="t('layout.sections.admin')"
        >
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
            <RouterLink
              v-if="hasActiveShelf && serverAdminAvailable"
              to="/admin/logs"
              class="sidebar-nav-item"
              exact-active-class="active"
            >
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
      <div v-if="showReadOnlyBanner" class="read-only-banner" role="status">
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
            <Icon name="menu" />
          </button>
          <h1 class="brand">
            <img class="brand-icon" :src="appIcon" alt="" aria-hidden="true">
            <span class="brand-name">{{ t('app.name') }}</span>
          </h1>
          <!-- On a narrow viewport the brand collapses to its icon and this
               takes the freed space to answer "where am I" — the current folder
               or page — which the full sidebar otherwise carries on wide. -->
          <span
            v-if="isNarrowViewport && currentLocationLabel"
            class="topbar-location"
            :title="currentLocationLabel"
          >{{ currentLocationLabel }}</span>
          <nav
            v-if="showHistoryControls"
            class="history-controls"
            :aria-label="t('layout.desktopHistoryNavigation')"
          >
            <button type="button" class="history-btn" :aria-label="t('layout.previousPage')" @click="goToPreviousPage">
              ←
            </button>
            <button type="button" class="history-btn" :aria-label="t('layout.nextPage')" @click="goToNextPage">
              →
            </button>
          </nav>
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

      <div class="page-area" :class="{ 'page-area-tabbar': isMobileShell }">
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

    <MobileTabBar v-if="isMobileShell" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter, type RouteLocationRaw } from 'vue-router';
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
import CreateFolderModal from '@/components/CreateFolderModal.vue';
import BookBatchProgressModal from '@/components/BookBatchProgressModal.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import Icon from '@/components/Icon.vue';
import FolderTree from '@/components/FolderTree.vue';
import MobileTabBar from '@/components/MobileTabBar.vue';
import RenameFolderModal from '@/components/RenameFolderModal.vue';
import TransferFolderModal from '@/components/TransferFolderModal.vue';
import SidebarNavIcon from '@/components/SidebarNavIcon.vue';
import { getBookshelfProvider, isMobileRuntime, isWailsRuntime } from '@/providers';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderManagement } from '@/composables/useFolderManagement';
import { useFolderStore } from '@/composables/useFolderStore';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useShelfPicker } from '@/composables/useShelfPicker';
import { useServerMode } from '@/composables/useServerMode';
import { useWriteAccess } from '@/composables/useWriteAccess';
import {
  RAIL_SIDEBAR_WIDTH,
  SIDEBAR_RESIZE_HIT_AREA_MARGINS,
  useSidebarLayout
} from '@/composables/useSidebarLayout';
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

// Both uses of this are the Downloads nav entry, so ask what that entry
// actually needs — a provider that keeps downloads — rather than which
// runtime we are on. Only MobileBookshelfProvider implements it, so this is
// the same answer by a name that says why.
const hasDownloadsStore = computed(() =>
  Boolean(getBookshelfProvider().listDownloadedBookEntries)
);

// The Wails desktop shell has a browser-history stack worth navigating; the web
// and mobile clients don't surface these pills. Scoping this to MainLayout keeps
// them off the immersive ReaderLayout routes, where the keyboard ←/→ already
// mean previous/next chapter.
const showHistoryControls = computed(() => isWailsRuntime());

// The mobile shell gets a bottom tab bar for its frequent destinations. Gated
// on the mobile *runtime* (not merely a narrow viewport) because the Downloads
// tab only exists on the mobile provider, and because a narrow desktop browser
// should keep the existing drawer. Latched like the runtime itself, so read it
// once rather than reactively.
const isMobileShell = isMobileRuntime();

function goToPreviousPage(): void {
  window.history.back();
}

function goToNextPage(): void {
  window.history.forward();
}
const { books, loading, fetchBooks } = useBookStore();
const { loading: foldersLoading, error: foldersError, loaded: foldersLoaded, fetchFolders } = useFolderStore();
const {
  moveBookError,
  showCreateFolderModal,
  creatingFolder,
  createFolderError,
  deleteFolderError,
  folderOperationError,
  pendingRenameFolderPath,
  renameFolderError,
  deletingFolderMap,
  pendingDeleteFolderPath,
  currentFolder,
  folderTree,
  canOpenFolder,
  isDeletingPendingFolder,
  pendingRenameFolderName,
  isRenamingPendingFolder,
  clearFolderErrors,
  onSelectFolder,
  openCreateFolderModal,
  closeCreateFolderModal,
  onSubmitCreateFolder,
  onMoveBook,
  requestRenameFolder,
  cancelPendingRenameFolder,
  confirmRenameFolder,
  canTransferFolder,
  transferFolderTarget,
  transferFolderName,
  transferFolderChain,
  transferFolderStatus,
  transferFolderPercentage,
  transferFolderError,
  transferFolderStarted,
  transferFolderRunning,
  transferFolderFinished,
  requestTransferFolder,
  cancelTransferFolder,
  submitTransferFolder,
  onMoveFolder,
  onOpenFolder,
  requestDeleteFolder,
  cancelPendingDeleteFolder,
  confirmDeleteFolder
} = useFolderManagement();
const { locale, setLocale, supportedLocales, t } = useI18n();
// The dropdown itself goes through useShelfPicker; what is left here is the
// resolved-shelf gate the rest of the layout hangs off, which is the same on
// every client.
const { loading: shelvesLoading, loaded: shelvesLoaded, selectedShelfID, ensureShelvesLoaded } = useShelvesStore();
const { fetchServerMode } = useServerMode();
const { writesEnabled, writeDisabledReason, libraryEditingAvailable, serverAdminAvailable } =
  useWriteAccess();
const readOnly = computed(() => !writesEnabled.value);
// The Android client being read-only is its normal state, not a condition to
// warn about, so the banner stays reserved for a server in read-only mode.
const showReadOnlyBanner = computed(() => writeDisabledReason.value === 'server-read-only');
const localeLabelKeyMap: Record<(typeof supportedLocales)[number], 'language.en' | 'language.zhHant'> = {
  en: 'language.en',
  'zh-Hant': 'language.zhHant'
};

const hasActiveShelf = computed(() => shelvesLoaded.value && selectedShelfID.value.length > 0);
const isSettingsRoute = computed(() => route.name === 'settings');
const canShowRouteContent = computed(() => isSettingsRoute.value || hasActiveShelf.value);
const shelfUnavailableMessage = computed(() =>
  shelvesLoading.value ? t('layout.shelf.loading') : t('layout.shelf.unavailableDescription')
);
// What the sidebar dropdown lists differs by client: the server's shelves on
// web and desktop, the device's own shelf list on the mobile shell.
const shelfPicker = useShelfPicker({
  onServerShelfSelected: async () => {
    clearFolderErrors();
    await Promise.all([fetchFolders(), fetchBooks()]);
    await router.push({ path: '/books', query: { page: '1' } });
  }
});

// The picker names where "manage shelves" points (or null for none); merge the
// current route query in so the mobile shell-preview flag survives the jump,
// while the picker's own query (the desktop settings tab) still wins.
const shelfManageTo = computed<RouteLocationRaw | null>(() => {
  const to = shelfPicker.manageTo;
  if (!to || typeof to === 'string') {
    return to;
  }
  return { ...to, query: { ...route.query, ...(to.query ?? {}) } };
});

// The narrow-viewport top bar shows where the user is instead of the language
// picker. Most MainLayout routes map to their existing sidebar label; the
// library route prefers the open folder's leaf name, and a book's detail page
// prefers the book's title, so the two cases the ticket cares about — "which
// folder", "which book" — read literally. Reader and source-edit live on
// ReaderLayout, so they never reach this bar.
const ROUTE_LOCATION_LABEL_KEYS: Record<string, string> = {
  home: 'layout.dashboard',
  library: 'layout.library',
  'read-history': 'layout.recentlyRead',
  trash: 'layout.trash',
  downloads: 'layout.downloads',
  'admin-logs': 'layout.adminLogs',
  settings: 'layout.settings',
  'duplicate-content': 'maintenance.duplicateContent',
  'similar-content': 'maintenance.similarContent',
  'not-found': 'notFound.title'
};

const currentFolderLeaf = computed(() => {
  const segments = (currentFolder.value ?? '').split('/').filter((segment) => segment.length > 0);
  return segments[segments.length - 1] ?? '';
});

const currentLocationLabel = computed(() => {
  const name = typeof route.name === 'string' ? route.name : '';

  if (name === 'library' && currentFolderLeaf.value) {
    return currentFolderLeaf.value;
  }

  if (name === 'book-detail') {
    const id = route.params.id;
    const book = typeof id === 'string' ? books.value.find((entry) => entry.id === id) : undefined;
    return book?.title ?? t('layout.library');
  }

  const key = ROUTE_LOCATION_LABEL_KEYS[name];
  return key ? t(key) : '';
});

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

  const nextID = value.trim();
  if (!nextID || nextID === shelfPicker.value.value) {
    return;
  }

  await shelfPicker.select(nextID);
}

onMounted(async () => {
  await fetchServerMode();
  await ensureShelvesLoaded();

  if (!hasActiveShelf.value) {
    return;
  }

  if (!foldersLoaded.value && !foldersLoading.value) {
    await fetchFolders();
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

.sidebar-folder-error {
  display: grid;
  gap: 8px;
  margin: 4px 8px;
}

.sidebar-folder-error p {
  margin: 0;
}

.sidebar-folder-error .button {
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

/* Sticks to the top of the viewport, so on the Android shell — which targets
   SDK 36, past the SDK 35 cutoff where edge-to-edge became mandatory — it sits
   under the status bar unless it carries the top inset itself. Insets are 0
   everywhere else, leaving the padding unchanged. */
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
  padding: calc(14px + env(safe-area-inset-top, 0px)) calc(24px + env(safe-area-inset-right, 0px))
    14px calc(24px + env(safe-area-inset-left, 0px));
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

.menu-btn svg {
  height: 18px;
  width: 18px;
}

.history-controls {
  align-items: center;
  display: inline-flex;
  gap: 6px;
}

.history-btn {
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
  line-height: 1;
  width: 38px;
}

.history-btn:hover {
  background: #ecf2f9;
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

.topbar-location {
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* The scrolling content reaches the bottom and side edges of the window, so it
   needs those insets to keep the last row — pagination, the mobile action bar's
   neighbours — clear of the gesture bar and of a landscape cutout. No top
   inset: .topbar sits above it inside the same scroller and already consumes
   that one, and adding it here would count it twice. */
.page-area {
  padding: 16px calc(24px + env(safe-area-inset-right, 0px))
    calc(16px + env(safe-area-inset-bottom, 0px)) calc(24px + env(safe-area-inset-left, 0px));
}

/* On the mobile shell a fixed bottom tab bar (MobileTabBar) overlays the
   viewport, so the last row of scrolled content needs room to clear it: the
   bar's own height plus the bottom inset it already carries. */
.page-area-tabbar {
  padding-bottom: calc(var(--mobile-tab-bar-height) + 16px + env(safe-area-inset-bottom, 0px));
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

/* Source type of a shelf, shown only on the mobile shell where one list can
   hold both PlainShelf servers and pCloud folders. */
.sidebar-shelf-type {
  color: var(--text-muted);
  font-size: 11px;
  margin-left: auto;
  padding-left: 8px;
}

.sidebar-shelf-manage {
  color: var(--text-muted);
  font-size: 12px;
  margin-top: 6px;
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
    /* Off-canvas and full-height, so it spans the status and gesture bars on
       the edge-to-edge Android shell. box-sizing is border-box globally, so
       this insets the content without widening the drawer. */
    padding: env(safe-area-inset-top, 0px) 0 env(safe-area-inset-bottom, 0px)
      env(safe-area-inset-left, 0px);
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
    padding: calc(10px + env(safe-area-inset-top, 0px)) calc(12px + env(safe-area-inset-right, 0px))
      10px calc(12px + env(safe-area-inset-left, 0px));
  }

  /* Language is a set-once preference; on a narrow screen it moves into the
     Settings page (its own tab) and the top bar spends that space on the
     brand-icon-plus-location pairing instead. The brand text collapses to the
     icon on a platform where the user already knows the app. */
  .brand-name {
    display: none;
  }

  .topbar-controls {
    display: none;
  }
}
</style>
