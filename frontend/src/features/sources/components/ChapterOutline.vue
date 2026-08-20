<template>
  <aside class="chapter-outline">
    <header>
      <div>
        <h3>{{ t('sources.chapters.title') }}</h3>
        <p class="meta">{{ t('sources.chapters.headingCount', { count: chapterSections.length }) }}</p>
      </div>
      <button class="button" type="button" :disabled="disabled" @click="emit('insert')">{{ t('sources.chapters.add') }}</button>
    </header>

    <ol class="chapter-list">
      <li :class="{ selected: !focused }">
        <button class="chapter-jump" type="button" @click="emit('showAll')">
          <span>{{ t('sources.chapters.allMarker') }}</span>
          {{ t('sources.chapters.wholeSource') }}
        </button>
      </li>

      <li
        v-if="openingSection && chapterSections.length > 0"
        :class="{ selected: activeSectionIndex === openingSection.index }"
      >
        <button class="chapter-jump" type="button" @click="emit('select', openingSection)">
          <span>—</span>
          {{ openingSection.title }}
        </button>
      </li>

      <li
        v-for="(section, index) in chapterSections"
        :key="`${section.startOffset}-${section.headingIndex}`"
        :class="{ selected: activeSectionIndex === section.index }"
      >
        <button class="chapter-jump" type="button" @click="emit('select', section)">
          <span>{{ index + 1 }}</span>
          {{ section.title }}
        </button>
        <div class="chapter-actions">
          <button
            class="text-action"
            type="button"
            :disabled="disabled"
            @click="emit('rename', section.headingIndex ?? index)"
          >{{ t('sources.chapters.rename') }}</button>
          <button
            class="text-action danger"
            type="button"
            :disabled="disabled"
            @click="emit('remove', section.headingIndex ?? index)"
          >{{ t('sources.chapters.merge') }}</button>
        </div>
      </li>
    </ol>
    <p v-if="chapterSections.length === 0" class="meta empty">{{ t('sources.chapters.empty') }}</p>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { MarkdownEditorSection } from '@/utils/markdownChapters';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const props = defineProps<{
  sections: MarkdownEditorSection[];
  focused: boolean;
  activeSectionIndex: number | null;
  disabled?: boolean;
}>();

const openingSection = computed(() => props.sections.find((section) => section.kind === 'opening'));
const chapterSections = computed(() => props.sections.filter((section) => section.kind === 'chapter'));

const emit = defineEmits<{
  insert: [];
  showAll: [];
  select: [section: MarkdownEditorSection];
  rename: [index: number];
  remove: [index: number];
}>();
</script>

<style scoped>
.chapter-outline { height: 100%; min-width: 0; overflow: auto; border-left: 1px solid var(--border); background: #fbfdff; }
.chapter-outline header { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 12px; border-bottom: 1px solid var(--border); }
.chapter-outline h3, .chapter-outline p { margin: 0; }
.chapter-list { list-style: none; padding: 8px; margin: 0; display: grid; gap: 7px; }
.chapter-list li { border: 1px solid var(--border); border-radius: 8px; background: white; }
.chapter-list li.selected { border-color: color-mix(in srgb, var(--accent) 62%, var(--border)); background: color-mix(in srgb, var(--accent) 8%, white); }
.chapter-jump { width: 100%; display: flex; gap: 8px; text-align: left; border: 0; background: transparent; padding: 9px; cursor: pointer; }
.chapter-jump span { color: var(--muted); min-width: 1.5rem; }
.chapter-actions { display: flex; justify-content: flex-end; gap: 8px; border-top: 1px solid var(--border); padding: 4px 8px; }
.text-action { border: 0; background: transparent; color: var(--accent); cursor: pointer; font-size: 12px; }
.text-action.danger { color: #b45309; }
.empty { padding: 2px 12px 10px; }
</style>
