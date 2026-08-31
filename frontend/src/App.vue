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
  <!--
    Outside RouterView so a toast raised by one page survives the navigation it
    may itself have caused, and so every route - library, reader, mobile shell -
    gets the same one host without mounting its own.
  -->
  <ToastHost />
</template>

<script setup lang="ts">
import { computed, onErrorCaptured, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { isMockApiMode } from '@/api/client';
import SecurityWarningBanner from '@/components/SecurityWarningBanner.vue';
import ToastHost from '@/components/ToastHost.vue';
import { useI18n } from './i18n';

const { t } = useI18n();
const route = useRoute();
const showMockModeBadge = computed(() => isMockApiMode());

// An uncaught render error unmounts the subtree and leaves a blank page with no
// way back. Capturing it here swaps in a recoverable panel instead. The hook
// deliberately returns nothing: returning false would stop propagation, and
// with it the app.config.errorHandler logging set up in main.ts.
const renderError = ref('');

onErrorCaptured((err) => {
  renderError.value = err instanceof Error ? err.message : String(err);
});

// A failed route is not necessarily fatal for the next one, so navigating away
// clears the panel and gives the app a chance to render again.
watch(
  () => route.fullPath,
  () => {
    renderError.value = '';
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
