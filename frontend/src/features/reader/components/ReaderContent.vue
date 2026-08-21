<template>
  <div ref="contentRef" class="reader-content" tabindex="-1" @scroll="emit('scroll')">
    <div class="reader-text">
      <template v-if="bookFormat === 'md'">
        <ReaderBlockWindow
          v-if="markdownBlocks.length > 0"
          :key="windowKey"
          :items="markdownBlocks"
          :estimate="estimateMarkdownBlockHeight"
        >
          <template #default="{ item }">
            <ReaderSafeHtml
              :html="item.html"
              :images="item.images"
              :book-id="bookId"
              :source-id="sourceId"
            />
          </template>
        </ReaderBlockWindow>
        <p v-else class="reader-text-block">{{ section?.text ?? '' }}</p>
      </template>

      <template v-else>
        <ReaderBlockWindow
          v-if="sectionBlocks.length > 0"
          :key="windowKey"
          :items="sectionBlocks"
          :estimate="estimateTextBlockHeight"
        >
          <template #default="{ item }">
            <component
              :is="item.type === 'quote' ? 'blockquote' : 'p'"
              class="reader-text-block"
              :class="{ 'reader-text-quote': item.type === 'quote' }"
            >{{ item.text }}</component>
          </template>
        </ReaderBlockWindow>
        <p v-else class="reader-text-block">{{ section?.text ?? '' }}</p>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import ReaderSafeHtml from '@/features/reader/components/ReaderSafeHtml.vue';
import ReaderBlockWindow from '@/features/reader/components/ReaderBlockWindow.vue';
import { renderMarkdownBlocks } from '@/utils/renderMarkdownBlocks';
import { parseReaderBlocks } from '@/features/reader/utils/parseReaderBlocks';
import {
  estimateMarkdownBlockHeight,
  estimateTextBlockHeight
} from '@/features/reader/utils/estimateBlockHeight';
import type { ReaderSection } from '@/types/book';

const props = defineProps<{
  bookId: string;
  sourceId: string;
  bookFormat: string;
  section: ReaderSection | null;
}>();

const emit = defineEmits<{
  scroll: [];
  ready: [element: HTMLDivElement | null];
}>();

const contentRef = ref<HTMLDivElement | null>(null);
const sectionBlocks = computed(() => parseReaderBlocks(props.section?.text ?? ''));
const markdownBlocks = computed(() => renderMarkdownBlocks(props.section?.text ?? ''));

// Remount the window when the shown source or section changes so its observer
// and placeholder heights start clean for the new block list.
const windowKey = computed(() => `${props.bookId}::${props.sourceId}::${props.section?.index ?? 0}`);

onMounted(() => emit('ready', contentRef.value));
onBeforeUnmount(() => emit('ready', null));
</script>

<style scoped src="../styles/reader-content.css"></style>
