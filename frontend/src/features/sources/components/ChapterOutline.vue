<template>
  <aside class="chapter-outline">
    <header>
      <div>
        <h3>Chapters</h3>
        <p class="meta">{{ headings.length }} H2 headings</p>
      </div>
      <button class="button" type="button" :disabled="disabled" @click="emit('insert')">Add</button>
    </header>

    <p v-if="hasOpening" class="opening-item">Opening</p>
    <p v-if="headings.length === 0" class="meta empty">No H2 chapters yet.</p>
    <ol v-else>
      <li v-for="(heading, index) in headings" :key="`${heading.startOffset}-${index}`">
        <button class="chapter-jump" type="button" @click="emit('jump', heading.startOffset)">
          <span>{{ index + 1 }}</span>
          {{ heading.title }}
        </button>
        <div class="chapter-actions">
          <button class="text-action" type="button" :disabled="disabled" @click="emit('rename', index)">Rename</button>
          <button class="text-action danger" type="button" :disabled="disabled" @click="emit('remove', index)">Merge</button>
        </div>
      </li>
    </ol>
  </aside>
</template>

<script setup lang="ts">
import type { MarkdownChapterHeading } from '@/features/reader/utils/markdownChapters';

defineProps<{
  headings: MarkdownChapterHeading[];
  hasOpening: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  insert: [];
  jump: [offset: number];
  rename: [index: number];
  remove: [index: number];
}>();
</script>

<style scoped>
.chapter-outline { height: 100%; min-width: 0; overflow: auto; border-left: 1px solid var(--border); background: #fbfdff; }
.chapter-outline header { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 12px; border-bottom: 1px solid var(--border); }
.chapter-outline h3, .chapter-outline p { margin: 0; }
.chapter-outline ol { list-style: none; padding: 8px; margin: 0; display: grid; gap: 7px; }
.chapter-outline li { border: 1px solid var(--border); border-radius: 8px; background: white; }
.chapter-jump { width: 100%; display: flex; gap: 8px; text-align: left; border: 0; background: transparent; padding: 9px; cursor: pointer; }
.chapter-jump span { color: var(--muted); min-width: 1.5rem; }
.chapter-actions { display: flex; justify-content: flex-end; gap: 8px; border-top: 1px solid var(--border); padding: 4px 8px; }
.text-action { border: 0; background: transparent; color: var(--accent); cursor: pointer; font-size: 12px; }
.text-action.danger { color: #b45309; }
.opening-item, .empty { padding: 10px 12px; }
.opening-item { color: var(--muted); border-bottom: 1px solid var(--border); }
</style>
