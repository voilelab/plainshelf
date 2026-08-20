<template>
  <!-- The single permitted v-html sink: nothing else may assign HTML to the DOM. -->
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div ref="root" v-html="sanitizedHtml" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { sanitizeHtml, type SafeHtmlProfile } from '@/utils/safeHtml';

const props = defineProps<{
  /** Untrusted renderer output; it is sanitized here, never before. */
  html: string;
  profile: SafeHtmlProfile;
}>();

const sanitizedHtml = computed(() => sanitizeHtml(props.html, props.profile));
const root = ref<HTMLDivElement | null>(null);

// The sanitized subtree is the only handle a wrapper gets on the rendered
// output; the reader uses it to find its illustration slots.
defineExpose({ root });
</script>
