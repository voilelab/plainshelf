<template>
  <div ref="contentRef" class="reader-content" tabindex="-1" @scroll="emit('scroll')">
    <h3 v-if="showChapterTitle && section?.title" class="reader-chapter-title">{{ section.title }}</h3>
    <div class="reader-text">
      <template v-if="bookFormat === 'md'">
        <template v-if="markdownBlocks.length > 0">
          <template v-for="(block, index) in markdownBlocks" :key="`${section?.index ?? 0}-md-${index}`">
            <template v-if="block.type === 'heading'">
              <component :is="mdHeadingTag(block.level)" :class="mdHeadingClass(block.level)">
                <template v-for="(segment, segmentIndex) in block.segments" :key="segmentIndex">
                  <strong v-if="segment.bold">{{ segment.text }}</strong>
                  <em v-else-if="segment.italic">{{ segment.text }}</em>
                  <code v-else-if="segment.code" class="reader-md-inline-code">{{ segment.text }}</code>
                  <template v-else>{{ segment.text }}</template>
                </template>
              </component>
            </template>

            <blockquote v-else-if="block.type === 'quote'" class="reader-text-block reader-text-quote">
              <template v-for="(segment, segmentIndex) in block.segments" :key="segmentIndex">
                <strong v-if="segment.bold">{{ segment.text }}</strong>
                <em v-else-if="segment.italic">{{ segment.text }}</em>
                <code v-else-if="segment.code" class="reader-md-inline-code">{{ segment.text }}</code>
                <template v-else>{{ segment.text }}</template>
              </template>
            </blockquote>

            <component v-else-if="block.type === 'list'" :is="block.ordered ? 'ol' : 'ul'" class="reader-md-list">
              <li v-for="(item, itemIndex) in block.items" :key="itemIndex">
                <template v-for="(segment, segmentIndex) in item" :key="segmentIndex">
                  <strong v-if="segment.bold">{{ segment.text }}</strong>
                  <em v-else-if="segment.italic">{{ segment.text }}</em>
                  <code v-else-if="segment.code" class="reader-md-inline-code">{{ segment.text }}</code>
                  <template v-else>{{ segment.text }}</template>
                </template>
              </li>
            </component>

            <pre v-else-if="block.type === 'code'" class="reader-md-code"><code>{{ block.text }}</code></pre>
            <hr v-else-if="block.type === 'hr'" class="reader-md-hr" />
            <ReaderAssetImage
              v-else-if="block.type === 'image'"
              :book-id="bookId"
              :source-id="sourceId"
              :name="block.name"
              :alt="block.alt"
            />

            <p v-else class="reader-text-block">
              <template v-for="(segment, segmentIndex) in block.segments" :key="segmentIndex">
                <strong v-if="segment.bold">{{ segment.text }}</strong>
                <em v-else-if="segment.italic">{{ segment.text }}</em>
                <code v-else-if="segment.code" class="reader-md-inline-code">{{ segment.text }}</code>
                <template v-else>{{ segment.text }}</template>
              </template>
            </p>
          </template>
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
import ReaderAssetImage from '@/features/reader/components/ReaderAssetImage.vue';
import { parseMarkdownBlocks } from '@/features/reader/utils/parseMarkdownBlocks';
import { parseReaderBlocks } from '@/features/reader/utils/parseReaderBlocks';
import type { ReaderSection } from '@/types/book';

const props = withDefaults(defineProps<{
  bookId: string;
  sourceId: string;
  bookFormat: string;
  section: ReaderSection | null;
  showChapterTitle?: boolean;
}>(), {
  showChapterTitle: true
});

const emit = defineEmits<{
  scroll: [];
  ready: [element: HTMLDivElement | null];
}>();

const contentRef = ref<HTMLDivElement | null>(null);
const sectionBlocks = computed(() => parseReaderBlocks(props.section?.text ?? ''));
const markdownBlocks = computed(() => parseMarkdownBlocks(props.section?.text ?? ''));

function mdHeadingTag(level: number): 'h2' | 'h3' | 'h4' {
  if (level === 1) return 'h2';
  if (level === 2) return 'h3';
  return 'h4';
}

function mdHeadingClass(level: number): string {
  if (level === 1) return 'reader-md-h1';
  if (level === 2) return 'reader-md-h2';
  return 'reader-md-h3';
}

onMounted(() => emit('ready', contentRef.value));
onBeforeUnmount(() => emit('ready', null));
</script>

<style scoped src="../styles/reader-content.css"></style>
