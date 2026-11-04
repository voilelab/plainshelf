import { computed, type ComputedRef, type Ref, type WritableComputedRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';

const ALWAYS_TABS = ['general', 'reading', 'shelves', 'system'];
const EDITABLE_TABS = ['import'];

export function useSettingsTabs(
  serverSettingsEditable: Ref<boolean> | ComputedRef<boolean>
): { activeSettingsTab: WritableComputedRef<string> } {
  const route = useRoute();
  const router = useRouter();

  const defaultSettingsTab = computed(() => (serverSettingsEditable.value ? 'general' : 'shelves'));

  const availableTabs = computed(() => [
    ...ALWAYS_TABS,
    ...(serverSettingsEditable.value ? EDITABLE_TABS : [])
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
