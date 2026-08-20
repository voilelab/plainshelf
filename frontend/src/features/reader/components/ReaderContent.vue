<template>
  <div ref="contentRef" class="reader-content" tabindex="-1" @scroll="emit('scroll')">
    <div class="reader-text">
      <template v-if="bookFormat === 'md'">
        <template v-if="markdownBlocks.length > 0">
          <ReaderSafeHtml
            v-for="(block, index) in markdownBlocks"
            :key="`${section?.index ?? 0}-md-${index}`"
            :html="block.html"
            :images="block.images"
            :book-id="bookId"
            :source-id="sourceId"
          />
        </template>
        <p v-else class="reader-text-block">{{ section?.text ?? '' }}</p>
      </template>

      <template v-else>
        <template v-if="sectionBlocks.length > 0">
          <component
            :is="block.type === 'quote' ? 'blockquote' : 'p'"
            v-for="(block, index) in sectionBlocks"
            :key="`${section?.index ?? 0}-${index}`"
            class="reader-text-block"
            :class="{ 'reader-text-quote': block.type === 'quote' }"
          >
            {{ block.text }}
          </component>
        </template>
        <p v-else class="reader-text-block">{{ section?.text ?? '' }}</p>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import ReaderSafeHtml from '@/features/reader/components/ReaderSafeHtml.vue';
import { renderMarkdownBlocks } from '@/utils/renderMarkdownBlocks';
import { parseReaderBlocks } from '@/features/reader/utils/parseReaderBlocks';
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

onMounted(() => emit('ready', contentRef.value));
onBeforeUnmount(() => emit('ready', null));
</script>

<style scoped src="../styles/reader-content.css"></style>
