<template>
  <div class="mobile-connect">
    <!-- Mounted only once the entry is resolved: the form reads it on mount to
         seed its fields, so an entry arriving later would never be picked up. -->
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

/**
 * Null while loading and for `/connect/new`. Read from storage rather than
 * passed through router state so the page survives a reload, which is how every
 * shelf switch lands.
 */
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
    await router.replace({ name: 'mobile-shelves', query: route.query });
    return;
  }
  entry.value = found;
  ready.value = true;
});

// Carries the query, for the reason spelled out in MobileShelvesPage.
function onCancel(): void {
  void router.push({ name: 'mobile-shelves', query: route.query });
}
</script>

<style scoped src="../styles/mobile-connect.css"></style>
