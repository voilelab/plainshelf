<template>
  <component :is="folderIconComponent(shown)" class="folder-icon" :data-folder-icon="shown" aria-hidden="true" />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { folderIconComponent, shownFolderIcon } from '@/components/icons/folderIcons';

// '' or a name this build does not ship draws the default folder icon.
const props = defineProps<{
  icon?: string;
}>();

const shown = computed(() => shownFolderIcon(props.icon));
</script>

<style scoped>
.folder-icon {
  flex: 0 0 auto;
  height: 16px;
  width: 16px;
}
</style>
