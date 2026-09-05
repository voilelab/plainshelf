import { computed, type ComputedRef, type Ref, type WritableComputedRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';

// Tabs that are always shown, plus the ones that render only when the server
// settings are editable (the cover, NSFW, EPUB-import and log-retention panels).
//
// `device-nsfw` is the third list: it is the device's own adult-content answer,
// which only a client with no server to ask acts on. Showing it beside the
// server's `nsfw` tab would put two switches for one question in front of a
// user whose server already decides it, so it appears exactly where that tab
// does not.
const ALWAYS_TABS = ['read-history', 'reader-launch', 'language', 'about', 'shelves'];
const EDITABLE_TABS = ['cover', 'nsfw', 'import', 'logs'];
const DEVICE_ONLY_TABS = ['device-nsfw'];

/**
 * The settings page's active tab, backed by the `?tab=` route query.
 *
 * Keeping the selection in the URL — rather than in a local ref synced by a
 * watcher — is what lets the sidebar's "manage shelves" deep link
 * (`/settings?tab=shelves`) select a tab even when the page is already mounted.
 * Crucially the setter writes the chosen tab back into the query on every
 * switch: without that, a user who deep-links to Shelves, switches to Cover,
 * then clicks the link again would re-target the still-current
 * `?tab=shelves` URL, Vue Router would emit no navigation, and the tab would
 * never return to Shelves.
 *
 * An unknown or unavailable `?tab=` value (e.g. `cover` on a read-only server)
 * falls back to the default tab rather than selecting a tab that is not shown.
 */
export function useSettingsTabs(
  serverSettingsEditable: Ref<boolean> | ComputedRef<boolean>
): { activeSettingsTab: WritableComputedRef<string> } {
  const route = useRoute();
  const router = useRouter();

  const defaultSettingsTab = computed(() => (serverSettingsEditable.value ? 'cover' : 'shelves'));

  const availableTabs = computed(() =>
    serverSettingsEditable.value
      ? [...ALWAYS_TABS, ...EDITABLE_TABS]
      : [...ALWAYS_TABS, ...DEVICE_ONLY_TABS]
  );

  function requestedTab(): string | null {
    const requested = route.query.tab;
    return typeof requested === 'string' && availableTabs.value.includes(requested)
      ? requested
      : null;
  }

  const activeSettingsTab = computed<string>({
    get: () => requestedTab() ?? defaultSettingsTab.value,
    set: (tab) => {
      if (tab === requestedTab()) {
        return;
      }
      // replace, not push: switching tabs should not stack history entries.
      void router.replace({ query: { ...route.query, tab } });
    }
  });

  return { activeSettingsTab };
}
