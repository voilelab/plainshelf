<template>
  <section class="panel settings-group">
    <h3>{{ t('settings.cover.title') }}</h3>
    <!-- The row stays one big <label> so clicking the description or the space
         beside it still toggles, as it did with the checkbox. The switch is a
         <button>, which is interactive content, so a click on the switch itself
         does not also run the label's activation behavior. `for` is what points
         the label at it; aria-labelledby keeps the accessible name the title
         alone rather than the whole row's text. -->
    <label class="setting-item" :for="switchId">
      <div>
        <span :id="labelId" class="setting-label">
          {{ t('settings.coverToJpg.label') }}
        </span>
        <p :id="descriptionId" class="setting-description">
          {{ t('settings.coverToJpg.description') }}
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

const switchId = `cover-to-jpg-${useId()}`;
const labelId = `cover-to-jpg-label-${useId()}`;
const descriptionId = `cover-to-jpg-description-${useId()}`;
</script>

<style scoped src="../styles/settings-form.css"></style>
