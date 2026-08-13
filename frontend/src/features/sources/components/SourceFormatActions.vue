<template>
  <div class="source-format-actions">
    <span class="format-badge" :class="{ legacy }">{{ legacy ? 'Legacy' : format.toUpperCase() }}</span>
    <template v-if="legacy">
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'legacy-upgrade')">Upgrade chapter format</button>
      <span class="meta">Creates a new Markdown source from the current legacy split result.</span>
    </template>
    <template v-else-if="format === 'txt'">
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'manual-md')">Manual TXT → MD</button>
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'regex-md')">Regex → MD</button>
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'line-count-md')">Fixed lines → MD</button>
    </template>
    <template v-else>
      <button class="button" type="button" :disabled="disabled" @click="emit('convert', 'plain-text')">Create plain-text source</button>
      <span class="meta">Heading hierarchy and chapter navigation will be lost.</span>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { SourceConversionKind } from '@/features/sources/components/SourceConversionModal.vue';

defineProps<{
  format: 'txt' | 'md';
  legacy: boolean;
  disabled: boolean;
}>();

const emit = defineEmits<{
  convert: [kind: SourceConversionKind];
}>();
</script>

<style scoped>
.source-format-actions {
  min-height: 42px;
  padding: 7px 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid var(--border);
  background: #fffdf7;
}

.format-badge {
  font-size: 12px;
  font-weight: 700;
  border-radius: 999px;
  padding: 3px 8px;
  background: #e0f2fe;
  color: #075985;
}

.format-badge.legacy {
  background: #fef3c7;
  color: #92400e;
}
</style>
