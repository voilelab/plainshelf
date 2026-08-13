<template>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="reader-safe-html" v-html="sanitizedHtml" />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { sanitizeReaderHtml } from '@/features/reader/utils/sanitizeReaderHtml';

const props = defineProps<{
  html: string;
}>();

const sanitizedHtml = computed(() => sanitizeReaderHtml(props.html));
</script>
