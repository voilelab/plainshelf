<template>
  <!--
    role="status", not "alert": the failure itself is already announced by the
    page's own alert region, and this only adds the number to quote for it.
    Fixed-position so a page with no reference to show keeps its exact layout.
  -->
  <div v-if="incident" class="error-incident" role="status" aria-live="polite">
    <span class="error-incident__label">{{ t('errorIncident.label') }}</span>
    <code class="error-incident__id">{{ incident }}</code>
    <button type="button" class="error-incident__copy" @click="copy">
      {{ copied ? t('errorIncident.copied') : t('errorIncident.copy') }}
    </button>
    <button
      type="button"
      class="error-incident__dismiss"
      :aria-label="t('errorIncident.dismiss')"
      :title="t('errorIncident.dismiss')"
      @click="dismissIncident"
    >
      ✕
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useErrorIncident } from '@/composables/useErrorIncident';
import { useI18n } from '@/i18n';

const { t } = useI18n();
const { incident, dismissIncident } = useErrorIncident();

const copied = ref(false);

// A second failure reuses the notice, so the confirmation has to belong to the
// number now shown rather than to the last one copied.
watch(incident, () => {
  copied.value = false;
});

async function copy(): Promise<void> {
  // No clipboard API (an insecure origin, an old WebView): the number is
  // selectable text either way, so the button simply reports nothing.
  if (!navigator.clipboard) {
    return;
  }

  try {
    await navigator.clipboard.writeText(incident.value);
    copied.value = true;
  } catch {
    copied.value = false;
  }
}
</script>

<style scoped>
.error-incident {
  position: fixed;
  left: 16px;
  bottom: 16px;
  z-index: var(--z-toast);
  display: flex;
  align-items: center;
  gap: 8px;
  /* 100vw is in visual pixels, which the zoomed shell has already scaled;
     divide it back out, as .reka-toast-viewport does. */
  max-width: min(360px, calc(100vw / var(--app-zoom, 1) - 32px));
  flex-wrap: wrap;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.14);
  color: var(--text);
  padding: 8px 10px;
  font-size: 12px;
}

.error-incident__label {
  color: var(--muted);
}

.error-incident__id {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  letter-spacing: 0.08em;
  user-select: all;
}

.error-incident__copy,
.error-incident__dismiss {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: inherit;
  cursor: pointer;
  padding: 2px 8px;
  font-size: 12px;
}

.error-incident__dismiss {
  border-color: transparent;
  color: var(--muted);
  padding: 2px 4px;
}
</style>
