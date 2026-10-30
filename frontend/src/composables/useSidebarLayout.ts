import { computed, nextTick, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import type { SplitterPanel } from 'reka-ui';
import { useIsNarrowViewport } from '@/composables/useViewport';
import {
  MIN_EXPANDED_SIDEBAR_WIDTH,
  getStoredSidebarExpandedWidth,
  getStoredSidebarMode,
  setStoredSidebarExpandedWidth,
  setStoredSidebarMode,
  type SidebarMode
} from '@/utils/sidebarMode';

// Reka UI 2.10.5 SplitterResizeHandle watches hitAreaMargins and re-registers the handle when its identity changes, including during a drag.
export const SIDEBAR_RESIZE_HIT_AREA_MARGINS = { coarse: 12, fine: 6 };
export const RAIL_SIDEBAR_WIDTH = 48;
const DEFAULT_EXPANDED_SIDEBAR_WIDTH = 240;
const SIDEBAR_SECTION_KEYS = ['folders', 'reading', 'maintenance', 'admin'] as const;
type SidebarSectionKey = (typeof SIDEBAR_SECTION_KEYS)[number];

export function useSidebarLayout() {
  const route = useRoute();

  const sidebarPanelRef = ref<InstanceType<typeof SplitterPanel> | null>(null);
  const sidebarMode = ref<SidebarMode>(getStoredSidebarMode());
  const isRailSidebar = computed(() => sidebarMode.value === 'rail');
  const lastExpandedSidebarWidth = ref<number | null>(getStoredSidebarExpandedWidth());
  const collapsedSidebarSections = reactive<Record<SidebarSectionKey, boolean>>({
    folders: false,
    reading: false,
    maintenance: false,
    admin: false
  });

  const isNarrowViewport = useIsNarrowViewport();
  const showRailNav = computed(() => isRailSidebar.value && !isNarrowViewport.value);
  const drawerOpen = ref(false);
  watch(() => route.fullPath, () => {
    drawerOpen.value = false;
  });
  watch(isNarrowViewport, (narrow) => {
    if (!narrow) {
      drawerOpen.value = false;
    }
  });

  async function toggleSidebarMode(): Promise<void> {
    if (sidebarMode.value === 'expanded') {
      const currentWidth = sidebarPanelRef.value?.getSize();
      if (typeof currentWidth === 'number' && currentWidth >= MIN_EXPANDED_SIDEBAR_WIDTH) {
        lastExpandedSidebarWidth.value = currentWidth;
        setStoredSidebarExpandedWidth(currentWidth);
      }
      sidebarMode.value = 'rail';
    } else {
      sidebarMode.value = 'expanded';
    }
    setStoredSidebarMode(sidebarMode.value);

    await applySidebarModeSize();
  }

  async function applySidebarModeSize(): Promise<void> {
    await nextTick();
    const panel = sidebarPanelRef.value;
    if (!panel) {
      return;
    }
    if (sidebarMode.value === 'rail') {
      panel.resize(RAIL_SIDEBAR_WIDTH);
      return;
    }

    // Reka UI 2.10.5 SplitterGroup debounces autosave by 100 ms.
    const expandedWidth = lastExpandedSidebarWidth.value;
    if (expandedWidth !== null) {
      panel.resize(expandedWidth);
    } else if ((panel.getSize() ?? 0) < MIN_EXPANDED_SIDEBAR_WIDTH) {
      panel.resize(DEFAULT_EXPANDED_SIDEBAR_WIDTH);
    }
  }

  function toggleSidebarSection(section: SidebarSectionKey): void {
    collapsedSidebarSections[section] = !collapsedSidebarSections[section];
  }

  return {
    sidebarPanelRef,
    isRailSidebar,
    showRailNav,
    isNarrowViewport,
    drawerOpen,
    collapsedSidebarSections,
    toggleSidebarMode,
    toggleSidebarSection
  };
}
