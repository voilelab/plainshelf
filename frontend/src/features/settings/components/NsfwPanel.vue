<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.nsfw.title') }}</h3>
    <!-- Same row-as-label shape as CoverPanel: the switch is a <button>, so
         `for` is what makes a click on the description toggle it too. -->
    <label class="setting-item" :for="switchId">
      <div>
        <span :id="labelId" class="setting-label">
          {{ t('settings.showNsfw.label') }}
        </span>
        <p :id="descriptionId" class="setting-description">
          {{ t('settings.showNsfw.description') }}
        </p>
      </div>
      <BaseSwitch
        :id="switchId"
        :model-value="value"
        :disabled="disabled"
        :aria-labelledby="labelId"
        :aria-describedby="descriptionId"
        @update:model-value="emit('change', $event)"
      />
    </label>
    <p class="setting-description settings-note">{{ t('settings.showNsfw.markingNote') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useId } from 'vue';
import BaseSwitch from '@/components/BaseSwitch.vue';
import { useI18n } from '@/i18n';

defineProps<{
  value: boolean;
  disabled: boolean;
}>();

const emit = defineEmits<{
  change: [value: boolean];
}>();

const { t } = useI18n();

const switchId = `show-nsfw-${useId()}`;
const labelId = `show-nsfw-label-${useId()}`;
const descriptionId = `show-nsfw-description-${useId()}`;
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
/* Not part of the switch's own row, so it must not inherit the row's layout —
   it explains where the marks themselves come from, which the switch does not
   control. */
.settings-note {
  margin: 0;
}
</style>
