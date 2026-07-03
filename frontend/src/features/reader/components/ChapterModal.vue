<template>
  <BaseDialog :open="open" title="Chapters" @close="emit('close')">
    <section class="panel chapter-modal">
      <header class="chapter-modal-header">
        <h3>Chapters</h3>
        <button class="chapter-icon-close" type="button" aria-label="Close chapter dialog" @click="emit('close')">×</button>
      </header>

      <div class="chapter-modal-list">
        <button
          v-for="section in sections"
          :key="section.index"
          class="chapter-modal-item"
          :class="{ active: section.index === currentSectionIndex }"
          type="button"
          @click="emit('selectSection', section.index)"
        >
          <span class="chapter-modal-item-index">{{ section.index + 1 }} / {{ sections.length }}</span>
          <span class="chapter-modal-item-title">{{ section.title }}</span>
        </button>
      </div>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import BaseDialog from '../../../components/BaseDialog.vue';

type ReaderSection = {
  index: number;
  title: string;
};

defineProps<{
  open: boolean;
  sections: ReaderSection[];
  currentSectionIndex: number;
}>();

const emit = defineEmits<{
  close: [];
  selectSection: [index: number];
}>();
</script>

<style scoped src="../styles/reader-modal.css"></style>
