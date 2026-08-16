<template>
  <BaseDialog :open="open" :title="t('reader.split.title')" :busy="savingSplit" @close="onClose">
    <section class="panel split-modal">
      <header class="split-header">
        <h3>{{ t('reader.split.title') }}</h3>
        <button class="icon-close" type="button" :aria-label="t('reader.split.closeLabel')" :disabled="savingSplit" @click="onClose">
          ×
        </button>
      </header>

      <p class="split-desc">{{ t('reader.split.description') }}</p>

      <div v-if="splitModalError" class="error" role="alert">{{ splitModalError }}</div>

      <form class="split-form" @submit.prevent="onSubmitSplitConfig">
        <label class="field">
          <span class="label">{{ t('reader.split.typeLabel') }}</span>
          <SelectRoot :model-value="draftType" :disabled="savingSplit" @update:model-value="onDraftTypeSelect">
            <SelectTrigger class="input select-trigger">
              <SelectValue />
            </SelectTrigger>
            <SelectPortal>
              <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                <SelectViewport>
                  <SelectItem v-for="opt in SPLIT_TYPE_OPTIONS" :key="opt" class="reka-menu-item" :value="opt">
                    <SelectItemText>{{ splitTypeLabel(opt) }}</SelectItemText>
                  </SelectItem>
                </SelectViewport>
              </SelectContent>
            </SelectPortal>
          </SelectRoot>
        </label>

        <label v-if="draftType === 'line_count'" class="field">
          <span class="label">{{ t('reader.split.lineCountLabel') }}</span>
          <input
            v-model="draftLineCount"
            class="input"
            type="number"
            min="1"
            step="1"
            :placeholder="t('reader.split.lineCountPlaceholder')"
            :disabled="savingSplit"
          />
        </label>

        <label v-if="draftType === 'regex'" class="field">
          <span class="label">{{ t('reader.split.regexLabel') }}</span>
          <textarea
            v-model="draftRegex"
            class="input split-textarea"
            rows="4"
            :placeholder="t('reader.split.regexPlaceholder')"
            :disabled="savingSplit"
          />
        </label>

        <label v-if="draftType === 'boundary'" class="field">
          <span class="label">{{ t('reader.split.boundaryLabel') }}</span>
          <textarea
            v-model="draftBoundaries"
            class="input split-textarea"
            rows="4"
            :placeholder="t('reader.split.boundaryPlaceholder')"
            :disabled="savingSplit"
          />
        </label>

        <div class="actions">
          <button class="button" type="button" :disabled="savingSplit" @click="onClose">{{ t('common.cancel') }}</button>
          <button class="button primary" type="submit" :disabled="savingSplit">
            {{ savingSplit ? t('reader.split.saving') : t('reader.split.save') }}
          </button>
        </div>
      </form>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
import BaseDialog from '@/components/BaseDialog.vue';
import type { SplitConfig, SplitType } from '@/types/book';
import { useI18n } from '@/i18n';

const { t } = useI18n();

// The option values are API tokens ('none', 'line_count', ...), which the
// Select used to render straight to the user.
function splitTypeLabel(type: string): string {
  return t(`reader.split.types.${type}`);
}

const SPLIT_TYPE_OPTIONS: SplitType[] = ['none', 'line_count', 'regex', 'boundary'];

const props = defineProps<{
  open: boolean;
  splitConfig: SplitConfig;
}>();

const emit = defineEmits<{
  close: [];
  saved: [config: SplitConfig];
}>();

const savingSplit = ref(false);
const splitModalError = ref('');
const draftType = ref<SplitType>('none');
const draftLineCount = ref('100');
const draftRegex = ref('');
const draftBoundaries = ref('1');

function hydrateSplitDraft(config: SplitConfig): void {
  draftType.value = config.type;
  draftLineCount.value = String(config.line_count ?? 100);
  draftRegex.value = config.regex ?? '';
  draftBoundaries.value = (config.boundaries ?? []).join(', ');
}

function buildDraftSplitConfig(): SplitConfig {
  if (draftType.value === 'line_count') {
    const parsed = Number.parseInt(draftLineCount.value, 10);
    return {
      type: 'line_count',
      line_count: Number.isNaN(parsed) ? 0 : parsed
    };
  }

  if (draftType.value === 'regex') {
    return {
      type: 'regex',
      regex: draftRegex.value
    };
  }

  if (draftType.value === 'boundary') {
    const boundaries = draftBoundaries.value
      .split(/[\s,]+/)
      .map((token) => Number.parseInt(token, 10))
      .filter((num) => !Number.isNaN(num));

    return {
      type: 'boundary',
      boundaries
    };
  }

  return { type: 'none' };
}

function onDraftTypeSelect(value: AcceptableValue): void {
  if (typeof value === 'string' && SPLIT_TYPE_OPTIONS.includes(value as SplitType)) {
    draftType.value = value as SplitType;
  }
}

function onClose(): void {
  if (savingSplit.value) {
    return;
  }
  emit('close');
}

function onSubmitSplitConfig(): void {
  savingSplit.value = true;
  splitModalError.value = '';

  try {
    emit('saved', buildDraftSplitConfig());
  } catch (err) {
    splitModalError.value = err instanceof Error ? err.message : t('reader.split.saveFailed');
  } finally {
    savingSplit.value = false;
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }
    hydrateSplitDraft(props.splitConfig);
    splitModalError.value = '';
  },
  { immediate: true }
);
</script>

<style scoped src="../styles/reader-modal.css"></style>
