<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.deviceNsfw.title') }}</h3>
    <!-- Same row-as-label shape as NsfwPanel: the switch is a <button>, so
         `for` is what makes a click on the description toggle it too. -->
    <label class="setting-item" :for="switchId">
      <div>
        <span :id="labelId" class="setting-label">
          {{ t('settings.deviceNsfw.label') }}
        </span>
        <p :id="descriptionId" class="setting-description">
          {{ t('settings.deviceNsfw.description') }}
        </p>
      </div>
      <BaseSwitch
        :id="switchId"
        :model-value="value"
        :aria-labelledby="labelId"
        :aria-describedby="descriptionId"
        @update:model-value="emit('change', $event)"
      />
    </label>
    <p class="setting-description settings-note">{{ t('settings.deviceNsfw.scopeNote') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useId } from 'vue';
import BaseSwitch from '@/components/BaseSwitch.vue';
import { useI18n } from '@/i18n';

// No `disabled` prop, unlike the server panels: this preference is written to
// localStorage, so it is never waiting on a request and there is nothing to
// disable it for.
defineProps<{
  value: boolean;
}>();

const emit = defineEmits<{
  change: [value: boolean];
}>();

const { t } = useI18n();

const switchId = `device-nsfw-${useId()}`;
const labelId = `device-nsfw-label-${useId()}`;
const descriptionId = `device-nsfw-description-${useId()}`;
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
/* Not part of the switch's own row: it says where this answer applies, which
   the switch itself does not control. */
.settings-note {
  margin: 0;
}
</style>
