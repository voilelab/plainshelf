<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.demoStorage.title') }}</h3>
    <p class="setting-description">{{ t('settings.demoStorage.description') }}</p>
    <div class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.demoStorage.statusLabel') }}</div>
        <p class="setting-description" data-testid="demo-storage-status" role="status">{{ statusText }}</p>
      </div>
      <button
        v-if="state === 'best-effort'"
        class="button primary"
        type="button"
        :disabled="requesting"
        @click="request"
      >
        {{ t('settings.demoStorage.request') }}
      </button>
    </div>
    <p v-if="declined && state === 'best-effort'" class="setting-description" role="status">
      {{ t('settings.demoStorage.declined') }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from '@/i18n';
import { useDemoStoragePersistence } from '@/features/settings/composables/useDemoStoragePersistence';

const { t } = useI18n();
const { state, requesting, declined, request } = useDemoStoragePersistence();

const statusText = computed(() => {
  switch (state.value) {
    case 'loading':
      return '…';
    case 'memory':
      return t('settings.demoStorage.statusMemory');
    case 'unsupported':
      return t('settings.demoStorage.statusUnsupported');
    case 'persisted':
      return t('settings.demoStorage.statusPersisted');
    default:
      return t('settings.demoStorage.statusBestEffort');
  }
});
</script>

<style scoped src="../styles/settings-form.css"></style>
