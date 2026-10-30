<template>
  <CollapsibleRoot
    as="div"
    class="similarity-filter"
    :open="advancedOpen"
    @update:open="emit('update:advancedOpen', $event)"
  >
    <div class="toolbar-bar similarity-tier-bar">
      <span class="toolbar-label">{{ t('maintenance.similar.tiersLabel') }}</span>
      <ToggleGroupRoot
        type="single"
        class="similarity-segmented"
        :aria-label="t('maintenance.similar.tiersLabel')"
        :model-value="advancedOpen ? SEGMENT_NONE : tier"
        @update:model-value="onSelectTierModel"
      >
        <ToggleGroupItem
          v-for="tierOption in tiers"
          :key="tierOption.key"
          :value="tierOption.key"
          class="toolbar-control toolbar-button similarity-segment"
        >
          {{ t(tierOption.labelKey) }}
        </ToggleGroupItem>
      </ToggleGroupRoot>
      <CollapsibleTrigger class="toolbar-control toolbar-button toolbar-small similarity-advanced-toggle">
        {{ t('maintenance.similar.advanced') }}
      </CollapsibleTrigger>
    </div>

    <CollapsibleContent class="toolbar-bar similarity-advanced">
      <span :id="SLIDER_LABEL_ID" class="toolbar-label">{{ t('maintenance.similar.thresholdLabel') }}</span>
      <SliderRoot
        class="slider-root similarity-slider"
        :model-value="[threshold]"
        :min="SIMILARITY_SLIDER_MIN"
        :max="SIMILARITY_SLIDER_MAX"
        :step="SIMILARITY_SLIDER_STEP"
        @update:model-value="onSlider"
      >
        <SliderTrack class="slider-track">
          <SliderRange class="slider-range" />
        </SliderTrack>
        <!-- Reka UI 2.10.5 SliderThumb defaults to a span with role=slider; HTML spans are not labelable elements. -->
        <SliderThumb
          :id="SLIDER_ID"
          class="slider-thumb"
          :aria-labelledby="SLIDER_LABEL_ID"
        />
      </SliderRoot>
      <span class="toolbar-label similarity-threshold-value">{{ thresholdPercent }}%</span>
      <span class="toolbar-label similarity-diff-readout">
        {{ t('maintenance.similar.diffReadout', { count: diffPer100 }) }}
      </span>
    </CollapsibleContent>

    <div class="toolbar-bar similarity-subset-bar">
      <label class="similarity-subset">
        <input
          type="checkbox"
          :checked="subsetOnly"
          @change="emit('update:subsetOnly', ($event.target as HTMLInputElement).checked)"
        />
        <span>{{ t('maintenance.similar.subsetToggle') }}</span>
      </label>
    </div>
  </CollapsibleRoot>
</template>

<script setup lang="ts">
import {
  CollapsibleContent,
  CollapsibleRoot,
  CollapsibleTrigger,
  SliderRange,
  SliderRoot,
  SliderThumb,
  SliderTrack,
  ToggleGroupItem,
  ToggleGroupRoot,
  type AcceptableValue
} from 'reka-ui';
import { computed } from 'vue';

import { useI18n } from '@/i18n';
import {
  approxDiffPer100Chars,
  SIMILARITY_SLIDER_MAX,
  SIMILARITY_SLIDER_MIN,
  SIMILARITY_SLIDER_STEP,
  SIMILARITY_TIERS,
  tierThreshold,
  type SimilarityTierKey
} from '@/utils/similarity';
import '@/styles/numeric-controls.css';
import '@/styles/toolbar-controls.css';

const SLIDER_ID = 'similarity-threshold-slider';
const SLIDER_LABEL_ID = 'similarity-threshold-label';
// Reka UI 2.10.5 single-value ToggleGroup selects passive mode from an initially undefined model; VueUse 14.3.0 still watches incoming props.
const SEGMENT_NONE = null;

const props = defineProps<{
  tier: SimilarityTierKey;
  advancedOpen: boolean;
  threshold: number;
  subsetOnly: boolean;
}>();

const emit = defineEmits<{
  (event: 'update:tier', tier: SimilarityTierKey): void;
  (event: 'update:advancedOpen', open: boolean): void;
  (event: 'update:threshold', threshold: number): void;
  (event: 'update:subsetOnly', subsetOnly: boolean): void;
}>();

const { t } = useI18n();

const tiers = SIMILARITY_TIERS;

const thresholdPercent = computed(() => Math.round(props.threshold * 100));
const diffPer100 = computed(() => approxDiffPer100Chars(props.threshold));

// Reka UI 2.10.5 single-value ToggleGroup deselects an active item by emitting undefined.
function onSelectTierModel(value: AcceptableValue): void {
  if (value == null) {
    return;
  }
  onSelectTier(value as SimilarityTierKey);
}

function onSelectTier(key: SimilarityTierKey): void {
  emit('update:tier', key);
  if (props.advancedOpen) {
    emit('update:advancedOpen', false);
  }
  emit('update:threshold', tierThreshold(key));
}

function onSlider(value: number[] | undefined): void {
  const next = value?.[0];
  if (next === undefined) {
    return;
  }
  emit('update:threshold', next);
}
</script>

<style scoped>
.similarity-filter {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.similarity-tier-bar,
.similarity-advanced,
.similarity-subset-bar {
  flex-wrap: wrap;
}

/* reka keeps the CollapsibleContent wrapper mounted and only marks it hidden
   when closed; `.toolbar-bar` sets display:flex, which would beat the
   user-agent [hidden] rule and leave an empty row taking column gap. */
.similarity-advanced[hidden] {
  display: none;
}

.similarity-segmented {
  display: inline-flex;
  gap: 0;
}

.similarity-segment {
  border-radius: 0;
}

.similarity-segment:first-child {
  border-top-left-radius: 6px;
  border-bottom-left-radius: 6px;
}

.similarity-segment:last-child {
  border-top-right-radius: 6px;
  border-bottom-right-radius: 6px;
}

.similarity-segment + .similarity-segment {
  margin-left: -1px;
}

.similarity-segment[data-state='on'],
.similarity-advanced-toggle[data-state='open'] {
  background: var(--accent-soft, #e6f0ff);
  border-color: var(--accent, #3b82f6);
  color: var(--accent, #2563eb);
  font-weight: 600;
  position: relative;
  z-index: 1;
}

.similarity-slider {
  flex: 1 1 200px;
  max-width: 320px;
}

.similarity-threshold-value {
  font-variant-numeric: tabular-nums;
  min-width: 3ch;
}

.similarity-diff-readout {
  color: var(--muted, #888);
  white-space: nowrap;
}

.similarity-subset {
  align-items: center;
  cursor: pointer;
  display: inline-flex;
  gap: 6px;
}
</style>
