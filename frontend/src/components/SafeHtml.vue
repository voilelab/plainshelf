<template>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div ref="root" v-html="sanitizedHtml" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { sanitizeHtml, type SafeHtmlProfile } from '@/utils/safeHtml';

const props = defineProps<{
  html: string;
  profile: SafeHtmlProfile;
}>();

const sanitizedHtml = computed(() => sanitizeHtml(props.html, props.profile));
const root = ref<HTMLDivElement | null>(null);

defineExpose({ root });
</script>
