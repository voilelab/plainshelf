<template>
  <div class="shelf-source-picker">
    <p class="shelf-source-prompt">{{ prompt }}</p>
    <section v-for="group in visibleGroups" :key="group.label" class="shelf-source-group">
      <h3 class="shelf-source-group-label">{{ group.label }}</h3>
      <div class="shelf-source-tiles">
        <button
          v-for="option in group.options"
          :key="option.value"
          type="button"
          class="shelf-source-tile"
          :data-testid="option.testId"
          :disabled="disabled"
          @click="emit('pick', option.value)"
        >
          <span class="shelf-source-tile-icon" :class="{ local: group.local }">
            <Icon :name="option.icon" />
          </span>
          <span class="shelf-source-tile-copy">
            <strong>{{ option.label }}</strong>
            <span>{{ option.help }}</span>
          </span>
          <Icon name="chevron-right" class="shelf-source-tile-chevron" />
        </button>
      </div>
    </section>
    <button
      v-if="shortcut"
      type="button"
      class="shelf-source-shortcut"
      :data-testid="shortcut.testId"
      :disabled="disabled"
      @click="emit('pick', shortcut.value)"
    >
      <Icon name="qrcode" class="shelf-source-shortcut-icon" />
      <span class="shelf-source-tile-copy">
        <strong>{{ shortcut.label }}</strong>
        <span>{{ shortcut.help }}</span>
      </span>
    </button>
  </div>
</template>

<script setup lang="ts" generic="T extends string">
import { computed } from 'vue';
import Icon from '@/components/Icon.vue';
import type { IconName } from '@/components/icons/registry';

interface Option {
  value: T;
  label: string;
  help: string;
  icon: IconName;
  testId?: string;
}

const props = defineProps<{
  prompt: string;
  groups: ReadonlyArray<{ label: string; local?: boolean; options: ReadonlyArray<Option> }>;
  shortcut?: Omit<Option, 'icon'> | null;
  disabled?: boolean;
}>();

const emit = defineEmits<{ (event: 'pick', value: T): void }>();

const visibleGroups = computed(() => props.groups.filter((group) => group.options.length > 0));
</script>

<style scoped>
.shelf-source-picker {
  display: grid;
  gap: 14px;
}

.shelf-source-prompt {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}

.shelf-source-group {
  display: grid;
  gap: 8px;
}

.shelf-source-group-label {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.shelf-source-tiles {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
}

.shelf-source-tile,
.shelf-source-shortcut {
  align-items: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  color: var(--text);
  cursor: pointer;
  display: flex;
  font: inherit;
  gap: 12px;
  padding: 12px 14px;
  text-align: left;
  width: 100%;
}

.shelf-source-tile:hover:not(:disabled),
.shelf-source-shortcut:hover:not(:disabled) {
  border-color: var(--border-hover);
}

.shelf-source-tile:disabled,
.shelf-source-shortcut:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-source-tile-icon {
  align-items: center;
  background: var(--surface-sunken);
  border-radius: 10px;
  color: var(--text-secondary);
  display: flex;
  flex: none;
  height: 40px;
  justify-content: center;
  width: 40px;
}

.shelf-source-tile-icon.local {
  background: var(--accent-soft);
  color: var(--accent);
}

.shelf-source-tile-icon :deep(svg),
.shelf-source-shortcut-icon {
  height: 20px;
  width: 20px;
}

.shelf-source-tile-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.shelf-source-tile-copy strong {
  font-size: 15px;
}

.shelf-source-tile-copy span {
  color: var(--muted);
  font-size: 13px;
}

.shelf-source-tile-chevron {
  color: var(--text-faint);
  flex: none;
  height: 18px;
  margin-left: auto;
  width: 18px;
}

.shelf-source-shortcut {
  border-style: dashed;
  border-color: var(--border-hover);
}

.shelf-source-shortcut-icon {
  color: var(--accent);
  flex: none;
}

.shelf-source-shortcut strong {
  color: var(--accent);
}
</style>
