<template>
  <!-- Keep one component root so parent reader scoped styles are inherited. -->
  <div class="reader-safe-html">
    <SafeHtml ref="safeHtml" class="reader-safe-html-content" :html="html" profile="reader" />
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
import { nextTick, onMounted, ref, shallowRef, watch } from 'vue';
import SafeHtml from '@/components/SafeHtml.vue';
import ReaderAssetImage from '@/features/reader/components/ReaderAssetImage.vue';
import type { ReaderMarkdownAsset } from '@/utils/renderMarkdownBlocks';

const props = defineProps<{
  html: string;
  images: ReaderMarkdownAsset[];
  bookId: string;
  sourceId: string;
}>();

// Local illustrations stay reader-owned: the shared sink renders inert slots,
// and only this wrapper knows they stand for a book's asset directory.
const safeHtml = ref<InstanceType<typeof SafeHtml> | null>(null);
const imageSlots = shallowRef<Array<{ image: ReaderMarkdownAsset; target: HTMLSpanElement }>>([]);

async function refreshImageSlots(): Promise<void> {
  await nextTick();
  const root = safeHtml.value?.root;
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
watch([() => props.html, () => props.images], () => void refreshImageSlots(), { flush: 'post' });
</script>
