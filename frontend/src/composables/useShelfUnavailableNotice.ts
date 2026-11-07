import { computed, type Ref } from 'vue';
import { useRoute, type RouteLocationRaw } from 'vue-router';
import { useI18n } from '@/i18n';
import { getShell } from '@/providers/shell';

export function useShelfUnavailableNotice(loading: Ref<boolean>) {
  const { t } = useI18n();
  const route = useRoute();

  const unavailable = computed(() => getShell()?.shelfUnavailable?.() ?? null);

  const message = computed(() => {
    if (loading.value) {
      return t('layout.shelf.loading');
    }
    return t(unavailable.value?.messageKey ?? 'layout.shelf.unavailableDescription');
  });

  const actionLabel = computed(() => t(unavailable.value?.actionKey ?? 'layout.settings'));

  const actionTo = computed<RouteLocationRaw>(() => {
    const to = unavailable.value?.fixTo ?? { path: '/settings' };
    if (typeof to === 'string') {
      return to;
    }
    return { ...to, query: { ...route.query, ...(to.query ?? {}) } };
  });

  return { message, actionLabel, actionTo };
}
