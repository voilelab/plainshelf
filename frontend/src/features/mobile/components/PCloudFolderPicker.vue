<template>
  <BaseDialog
    :open="open"
    :title="t('mobileConnect.pcloud.picker.title')"
    :described-by="descriptionId"
    @close="emit('close')"
  >
    <section class="panel pcloud-picker">
      <header class="pcloud-picker-header">
        <h2>{{ t('mobileConnect.pcloud.picker.title') }}</h2>
        <button
          class="pcloud-picker-close"
          type="button"
          :aria-label="t('common.closeDialog')"
          @click="emit('close')"
        >
          ×
        </button>
      </header>

      <nav class="pcloud-picker-crumbs" :aria-label="t('mobileConnect.pcloud.picker.breadcrumbLabel')">
        <button type="button" class="pcloud-picker-crumb" @click="goTo(-1)">
          {{ t('mobileConnect.pcloud.picker.rootLabel') }}
        </button>
        <template v-for="(level, index) in trail" :key="level.folderid">
          <span class="pcloud-picker-crumb-separator" aria-hidden="true">›</span>
          <button
            type="button"
            class="pcloud-picker-crumb"
            :aria-current="index === trail.length - 1 ? 'true' : undefined"
            @click="goTo(index)"
          >
            {{ level.name }}
          </button>
        </template>
      </nav>

      <p v-if="notice" class="pcloud-picker-notice" role="status">{{ notice }}</p>

      <div class="pcloud-picker-list">
        <button
          v-if="trail.length > 0"
          type="button"
          class="pcloud-picker-row pcloud-picker-up"
          @click="goTo(trail.length - 2)"
        >
          {{ t('mobileConnect.pcloud.picker.up') }}
        </button>

        <p v-if="loading" class="pcloud-picker-note" role="status">
          {{ t('mobileConnect.pcloud.picker.loading') }}
        </p>

        <template v-else-if="!error">
          <button
            v-for="folder in folders"
            :key="folder.folderid"
            type="button"
            class="pcloud-picker-row"
            @click="enter(folder)"
          >
            <span class="pcloud-picker-row-name">{{ folder.name }}</span>
            <span class="pcloud-picker-row-chevron" aria-hidden="true">›</span>
          </button>

          <p v-if="folders.length === 0" class="pcloud-picker-note">
            {{ t('mobileConnect.pcloud.picker.empty') }}
          </p>
        </template>

        <template v-else>
          <p class="pcloud-picker-error" role="alert">{{ error }}</p>
          <button type="button" class="button" @click="retry">
            {{ t('mobileConnect.pcloud.picker.retry') }}
          </button>
        </template>
      </div>

      <footer class="pcloud-picker-footer">
        <p :id="descriptionId" class="pcloud-picker-path">
          {{ t('mobileConnect.pcloud.picker.currentPath', { path: currentPath }) }}
        </p>
        <p class="pcloud-picker-verdict" :class="{ 'is-shelf': isShelf }" role="status">
          {{
            isShelf
              ? t('mobileConnect.pcloud.picker.isShelf')
              : t('mobileConnect.pcloud.picker.notShelf')
          }}
        </p>
        <div class="pcloud-picker-actions">
          <button type="button" class="button" @click="emit('close')">
            {{ t('mobileConnect.pcloud.picker.cancel') }}
          </button>
          <button
            type="button"
            class="button primary"
            :disabled="!isShelf || loading"
            @click="emit('select', currentPath)"
          >
            {{ t('mobileConnect.pcloud.picker.confirm') }}
          </button>
        </div>
      </footer>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, useId, watch } from 'vue';

import BaseDialog from '@/components/BaseDialog.vue';
import { PCloudClient } from '@/api/pcloud/client';
import { useI18n } from '@/i18n';
import { usePCloudFolderBrowser } from '@/features/mobile/composables/usePCloudFolderBrowser';

const props = defineProps<{
  open: boolean;
  /** Regional API host of the authorized account. */
  host: string;
  accessToken: string;
  /** Where to start browsing; the account root when it does not resolve. */
  initialPath: string;
}>();

const emit = defineEmits<{
  (event: 'select', path: string): void;
  (event: 'close'): void;
}>();

const { t } = useI18n();

const descriptionId = `pcloud-picker-path-${useId()}`;

// Built per request rather than held: the form can re-authorize while the
// picker is closed, and a client captured once would keep using the old token.
// `open` is renamed: the prop of that name is what the template needs to see.
const {
  trail,
  folders,
  currentPath,
  isShelf,
  loading,
  error,
  notice,
  open: openAt,
  enter,
  goTo,
  retry,
  close
} = usePCloudFolderBrowser(
  () => new PCloudClient({ host: props.host, accessToken: props.accessToken })
);

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      void openAt(props.initialPath);
      return;
    }
    // Nothing is on screen any more, so a listing still in flight has nowhere
    // to land and would only spend the connection.
    close();
  },
  { immediate: true }
);

onBeforeUnmount(close);
</script>

<style scoped>
.pcloud-picker {
  /* Flex, not a fixed grid template: the notice line is conditional, and a
     template would have to name a row for it that shifts every other row when
     it is absent. Here the list is simply the one part that may shrink. */
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  width: min(480px, calc(100vw / var(--app-zoom, 1) - 32px));
  /* Capped rather than fixed: a folder with three children gets a short dialog,
     while a long listing scrolls inside the list instead of pushing the confirm
     button off the bottom of a phone screen. First line is the fallback for
     engines without dvh, as in mobile-connect.css. */
  max-height: min(560px, calc(100vh / var(--app-zoom, 1) - 64px));
  max-height: min(560px, calc(100dvh / var(--app-zoom, 1) - 64px));
  box-sizing: border-box;
}

.pcloud-picker-header {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 12px;
  justify-content: space-between;
}

.pcloud-picker-header h2 {
  font-size: 18px;
  line-height: 1.2;
  margin: 0;
}

.pcloud-picker-close {
  align-items: center;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  cursor: pointer;
  display: inline-flex;
  font-size: 20px;
  height: 32px;
  justify-content: center;
  line-height: 1;
  width: 32px;
}

.pcloud-picker-crumbs {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 4px;
  /* Deep paths scroll sideways instead of wrapping into a growing block that
     would eat the list's room. */
  overflow-x: auto;
  white-space: nowrap;
}

.pcloud-picker-crumb {
  background: transparent;
  border: 0;
  border-radius: 6px;
  color: var(--accent);
  cursor: pointer;
  font-size: 13px;
  padding: 4px 6px;
}

.pcloud-picker-crumb[aria-current='true'] {
  color: inherit;
  font-weight: 600;
}

.pcloud-picker-crumb-separator {
  color: var(--muted);
  font-size: 13px;
}

.pcloud-picker-notice {
  color: var(--muted);
  flex: 0 0 auto;
  font-size: 12px;
  margin: 0;
  overflow-wrap: anywhere;
}

.pcloud-picker-list {
  border: 1px solid var(--border);
  border-radius: 8px;
  display: flex;
  /* The one part that gives way when the dialog hits its cap. */
  flex: 1 1 auto;
  flex-direction: column;
  gap: 2px;
  /* So the box does not collapse to nothing while a level is loading or when a
     folder turns out to be empty. */
  min-height: 96px;
  overflow-y: auto;
  padding: 6px;
}

.pcloud-picker-row {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  gap: 8px;
  justify-content: space-between;
  /* Comfortably tappable on a phone. */
  min-height: 44px;
  padding: 8px 10px;
  text-align: left;
  width: 100%;
}

.pcloud-picker-row:hover {
  background: rgba(31, 111, 235, 0.08);
}

.pcloud-picker-up {
  color: var(--muted);
}

.pcloud-picker-row-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pcloud-picker-row-chevron {
  color: var(--muted);
}

.pcloud-picker-note {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
  padding: 8px 10px;
}

.pcloud-picker-error {
  color: var(--danger);
  font-size: 13px;
  margin: 0;
  padding: 8px 10px;
}

.pcloud-picker-footer {
  display: grid;
  flex: 0 0 auto;
  gap: 8px;
}

.pcloud-picker-path {
  font-size: 13px;
  margin: 0;
  overflow-wrap: anywhere;
}

.pcloud-picker-verdict {
  color: var(--muted);
  font-size: 12px;
  margin: 0;
}

.pcloud-picker-verdict.is-shelf {
  color: var(--accent);
}

.pcloud-picker-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
</style>
