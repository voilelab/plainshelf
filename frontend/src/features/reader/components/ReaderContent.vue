<template>
  <div ref="contentRef" class="reader-content" tabindex="-1" @scroll="emit('scroll')">
    <div class="reader-text">
      <template v-if="bookFormat === 'md'">
        <ReaderBlockWindow
          v-if="markdownBlocks.length > 0"
          :key="windowKey"
          :items="markdownBlocks"
          :estimate="estimateMarkdownHeight"
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
          :estimate="estimateTextHeight"
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
import { renderMarkdownBlocks, type ReaderMarkdownBlock } from '@/utils/renderMarkdownBlocks';
import { parseReaderBlocks, type ReaderTextBlock } from '@/features/reader/utils/parseReaderBlocks';
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

// Placeholder heights for off-screen blocks. They only need to be in the right
// ballpark: a block is measured for real once it mounts, and the estimate keeps
// the scroll height stable enough that a restored offset lands close.
function charsToHeight(chars: number): number {
  const lines = Math.max(1, Math.ceil(chars / 40));
  return lines * 38 + 18;
}

function estimateMarkdownHeight(block: ReaderMarkdownBlock): number {
  return charsToHeight(block.html.replace(/<[^>]+>/g, '').length);
}

function estimateTextHeight(block: ReaderTextBlock): number {
  return charsToHeight(block.text.length);
}

onMounted(() => emit('ready', contentRef.value));
onBeforeUnmount(() => emit('ready', null));
</script>

<style scoped src="../styles/reader-content.css"></style>
