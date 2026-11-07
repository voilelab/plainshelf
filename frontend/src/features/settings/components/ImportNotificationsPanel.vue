<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.importNotifications.title') }}</h3>
    <label class="setting-item" :for="switchId">
      <div>
        <span :id="labelId" class="setting-label">
          {{ t('settings.importNotifications.label') }}
        </span>
        <p :id="descriptionId" class="setting-description">
          {{ t('settings.importNotifications.description') }}
        </p>
      </div>
      <BaseSwitch
        :id="switchId"
        :model-value="enabled"
        :aria-labelledby="labelId"
        :aria-describedby="descriptionId"
        @update:model-value="setEnabled"
      />
    </label>
  </section>
</template>

<script setup lang="ts">
import { useId } from 'vue';
import BaseSwitch from '@/components/BaseSwitch.vue';
import { useImportNotificationPreference } from '@/composables/useImportNotificationPreference';
import { useI18n } from '@/i18n';

const { t } = useI18n();
const { enabled, setEnabled } = useImportNotificationPreference();

const switchId = `import-notifications-${useId()}`;
const labelId = `import-notifications-label-${useId()}`;
const descriptionId = `import-notifications-description-${useId()}`;
</script>

<style scoped src="../styles/settings-form.css"></style>
