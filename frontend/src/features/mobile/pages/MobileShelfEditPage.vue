<template>
  <div class="mobile-shelf-edit">
    <MobileShelfForm v-if="ready" :entry="entry" cancellable @cancel="onCancel" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import MobileShelfForm from '@/features/mobile/components/MobileShelfForm.vue';
import { loadShelfEntries, type ShelfEntry } from '@/providers/mobileConfig';

const route = useRoute();
const router = useRouter();

const entry = ref<ShelfEntry | null>(null);
const ready = ref(false);

onMounted(async () => {
  const entryID = String(route.params.entryId ?? '');
  if (!entryID) {
    ready.value = true;
    return;
  }

  const { entries } = await loadShelfEntries();
  const found = entries.find((candidate) => candidate.id === entryID);
  if (!found) {
    await router.replace(manageShelvesRoute());
    return;
  }
  entry.value = found;
  ready.value = true;
});

function onCancel(): void {
  void router.push(manageShelvesRoute());
}

function manageShelvesRoute() {
  return { path: '/settings', query: { ...route.query, tab: 'shelves' } };
}
</script>

<style scoped>
.mobile-shelf-edit {
  width: 100%;
  display: flex;
  justify-content: center;
}
</style>
