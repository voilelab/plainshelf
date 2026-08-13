<template>
  <!-- Keep one component root so parent reader scoped styles are inherited. -->
  <div class="reader-safe-html">
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div ref="htmlRoot" class="reader-safe-html-content" v-html="sanitizedHtml" />
    <Teleport v-for="slot in imageSlots" :key="slot.image.token" :to="slot.target">
      <ReaderAssetImage
        :book-id="bookId"
        :source-id="sourceId"
        :name="slot.image.name"
        :alt="slot.image.alt"
      />
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, shallowRef, watch } from 'vue';
import ReaderAssetImage from '@/features/reader/components/ReaderAssetImage.vue';
import { sanitizeReaderHtml } from '@/features/reader/utils/sanitizeReaderHtml';
import type { ReaderMarkdownAsset } from '@/features/reader/utils/renderMarkdownBlocks';

const props = defineProps<{
  html: string;
  images: ReaderMarkdownAsset[];
  bookId: string;
  sourceId: string;
}>();

const sanitizedHtml = computed(() => sanitizeReaderHtml(props.html));
const htmlRoot = ref<HTMLDivElement | null>(null);
const imageSlots = shallowRef<Array<{ image: ReaderMarkdownAsset; target: HTMLSpanElement }>>([]);

async function refreshImageSlots(): Promise<void> {
  await nextTick();
  const root = htmlRoot.value;
  if (!root) {
    imageSlots.value = [];
    return;
  }

  const candidates = Array.from(root.querySelectorAll<HTMLSpanElement>('span.reader-asset-slot'));
  imageSlots.value = props.images.flatMap((image) => {
    const target = candidates.find((candidate) => candidate.title === image.token);
    if (!target) return [];
    // The title is a renderer-only locator. Remove it before mounting so the
    // opaque token never appears as a tooltip or an exposed reader attribute.
    target.removeAttribute('title');
    return [{ image, target }];
  });
}

onMounted(() => void refreshImageSlots());
watch([sanitizedHtml, () => props.images], () => void refreshImageSlots(), { flush: 'post' });
</script>
