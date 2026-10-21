<template>
  <section v-if="renderError" class="app-render-error panel" role="alert">
    <h2>{{ t('app.renderError.title') }}</h2>
    <p>{{ t('app.renderError.description') }}</p>
    <pre class="app-render-error-detail">{{ renderError }}</pre>
    <button type="button" class="button primary" @click="reload">
      {{ t('app.renderError.reload') }}
    </button>
  </section>
  <RouterView v-else />
  <div v-if="showMockModeBadge" class="mock-mode-badge" role="status" aria-live="polite">
    {{ t('app.mockModeBadge') }}
  </div>
  <SecurityWarningBanner />
  <ErrorIncidentNotice />
  <ToastHost />
</template>

<script setup lang="ts">
import { computed, onErrorCaptured, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { isMockApiMode } from '@/api/client';
import ErrorIncidentNotice from '@/components/ErrorIncidentNotice.vue';
import SecurityWarningBanner from '@/components/SecurityWarningBanner.vue';
import ToastHost from '@/components/ToastHost.vue';
import { useErrorIncident } from '@/composables/useErrorIncident';
import { useI18n } from './i18n';

const { t } = useI18n();
const route = useRoute();
const { dismissIncident } = useErrorIncident();
const showMockModeBadge = computed(() => isMockApiMode());

const renderError = ref('');

onErrorCaptured((err) => {
  renderError.value = err instanceof Error ? err.message : String(err);
});

watch(
  () => route.fullPath,
  () => {
    renderError.value = '';
    dismissIncident();
  }
);

function reload(): void {
  window.location.reload();
}
</script>

<style scoped>
.app-render-error {
  display: grid;
  gap: 10px;
  margin: 24px auto;
  max-width: 640px;
  padding: 24px;
  width: calc(100% - 48px);
}

.app-render-error h2,
.app-render-error p {
  margin: 0;
}

.app-render-error p {
  color: var(--muted);
}

.app-render-error-detail {
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 8px;
  margin: 0;
  overflow-x: auto;
  padding: 10px;
  font-size: 12px;
}

.app-render-error .button {
  justify-self: start;
}

.mock-mode-badge {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 1000;
  background: #7c2d12;
  color: #fff;
  border: 1px solid #9a3412;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  letter-spacing: 0.06em;
  font-weight: 700;
}
</style>
