import { computed, type ComputedRef, type Ref, type WritableComputedRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';

const ALWAYS_TABS = ['read-history', 'reader-launch', 'language', 'about', 'shelves'];
const EDITABLE_TABS = ['cover', 'nsfw', 'import', 'logs'];
const DEVICE_ONLY_TABS = ['device-nsfw'];
const DEMO_TABS = ['demo-storage'];

export function useSettingsTabs(
  serverSettingsEditable: Ref<boolean> | ComputedRef<boolean>,
  wasmDemo = false
): { activeSettingsTab: WritableComputedRef<string> } {
  const route = useRoute();
  const router = useRouter();

  const defaultSettingsTab = computed(() => (serverSettingsEditable.value ? 'cover' : 'shelves'));

  const availableTabs = computed(() => [
    ...ALWAYS_TABS,
    ...(serverSettingsEditable.value ? EDITABLE_TABS : DEVICE_ONLY_TABS),
    ...(wasmDemo ? DEMO_TABS : [])
  ]);

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
      void router.replace({ query: { ...route.query, tab } });
    }
  });

  return { activeSettingsTab };
}
