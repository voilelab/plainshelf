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

// Stable reference: an inline object literal in the template would be recreated on
// every MainLayout re-render, and reka-ui's SplitterResizeHandle re-registers itself
// with the group whenever its `hitAreaMargins` prop identity changes. Re-registering
// mid-drag replaces the handle's internal registry entry, so the `mouseup` handler's
// reference-based lookup misses and `stopDragging()` never fires — leaving
// `pointer-events: none` stuck on every panel. A module-level constant keeps the prop
// identity stable across renders and avoids the mid-drag re-registration entirely.
export const SIDEBAR_RESIZE_HIT_AREA_MARGINS = { coarse: 12, fine: 6 };
export const RAIL_SIDEBAR_WIDTH = 48;
const DEFAULT_EXPANDED_SIDEBAR_WIDTH = 240;
const SIDEBAR_SECTION_KEYS = ['folders', 'reading', 'maintenance', 'admin'] as const;
export type SidebarSectionKey = (typeof SIDEBAR_SECTION_KEYS)[number];

/**
 * Sidebar chrome for MainLayout: rail/expanded mode with its persisted width, the
 * narrow-viewport drawer, and per-section collapse. Owns no data — folders, books
 * and shelves stay with their own stores.
 */
export function useSidebarLayout() {
  const route = useRoute();

  const sidebarPanelRef = ref<InstanceType<typeof SplitterPanel> | null>(null);
  const sidebarMode = ref<SidebarMode>(getStoredSidebarMode());
  const isRailSidebar = computed(() => sidebarMode.value === 'rail');
  // Rail mode resizes the panel to 48px, which overwrites the splitter's
  // auto-saved layout, so the expanded width has to be persisted separately to
  // survive a reload while railed.
  const lastExpandedSidebarWidth = ref<number | null>(getStoredSidebarExpandedWidth());
  const collapsedSidebarSections = reactive<Record<SidebarSectionKey, boolean>>({
    folders: false,
    reading: false,
    maintenance: false,
    admin: false
  });

  // Narrow-viewport (mobile) drawer state. Every sidebar action ends in a
  // router.push, so watching fullPath closes the drawer after nav links and
  // folder selection alike.
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

    // Constraint props alone should re-clamp the panel, but nudge explicitly so
    // the width is deterministic even if reka's px re-validation misses a beat.
    await nextTick();
    const panel = sidebarPanelRef.value;
    if (!panel) {
      return;
    }
    if (sidebarMode.value === 'rail') {
      panel.resize(RAIL_SIDEBAR_WIDTH);
      return;
    }

    // Our stored width wins over whatever the splitter restores for the expanded
    // constraint set: a drag followed immediately by railing never reaches reka's
    // debounced auto-save, so it would otherwise fall back to the default width.
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
