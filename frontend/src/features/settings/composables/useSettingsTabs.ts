import { computed, type ComputedRef, type Ref, type WritableComputedRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';

const ALWAYS_TABS = ['shelves', 'general', 'reading', 'system'];
const EDITABLE_TABS = ['import'];
const DEFAULT_TAB = 'shelves';

export function useSettingsTabs(
  serverSettingsEditable: Ref<boolean> | ComputedRef<boolean>
): { activeSettingsTab: WritableComputedRef<string> } {
  const route = useRoute();
  const router = useRouter();

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
    get: () => requestedTab() ?? DEFAULT_TAB,
    set: (tab) => {
      if (tab === requestedTab()) {
        return;
      }
      void router.replace({ query: { ...route.query, tab } });
    }
  });

  return { activeSettingsTab };
}
