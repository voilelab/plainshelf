<template>
  <img :src="src" :alt="alt" @error="handleError" />
</template>

<script setup lang="ts">
import { useCoverSrc } from '@/composables/useCoverSrc';

const props = defineProps<{
  bookId: string;
  coverUrl?: string;
  alt: string;
  cacheKey?: number;
}>();

const { src, handleError } = useCoverSrc(
  () => props.bookId,
  () => props.coverUrl,
  () => props.cacheKey
);
</script>
