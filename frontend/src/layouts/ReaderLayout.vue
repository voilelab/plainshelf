<template>
  <div class="reader-layout">
    <RouterView v-if="hasActiveShelf" />
    <section v-else class="reader-no-shelf" role="status">
      <h2>{{ t('layout.shelf.unavailableTitle') }}</h2>
      <p>{{ shelfUnavailableMessage }}</p>
      <RouterLink to="/settings" class="button">{{ t('layout.settings') }}</RouterLink>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useI18n } from '@/i18n';

const { t } = useI18n();
// The same store MainLayout loads, rather than a private copy of the same
// resolution: it already retries a shelf that is still initializing and falls
// back to a device-persisted shelf when the list cannot be fetched, and it is
// what carries each shelf's read_only to the source editor, which a direct
// link opens without MainLayout ever mounting.
const {
  loading: shelvesLoading,
  loaded: shelvesLoaded,
  selectedShelfID,
  ensureShelvesLoaded
} = useShelvesStore();

const hasActiveShelf = computed(() => shelvesLoaded.value && selectedShelfID.value.length > 0);
const shelfUnavailableMessage = computed(() =>
  shelvesLoading.value ? t('layout.shelf.loading') : t('layout.shelf.unavailableDescription')
);

onMounted(() => {
  void ensureShelvesLoaded();
});
</script>

<style scoped>
.reader-layout {
  height: calc(100vh / var(--app-zoom, 1));
  width: calc(100vw / var(--app-zoom, 1));
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  box-sizing: border-box;
  background:
    radial-gradient(circle at top, rgba(255, 250, 240, 0.92), rgba(245, 241, 232, 0.96) 42%),
    linear-gradient(180deg, #f7f2e7 0%, #efe7d7 100%);
}

.reader-no-shelf {
  background: rgba(255, 255, 255, 0.86);
  border: 1px solid var(--border);
  border-radius: 12px;
  display: grid;
  gap: 10px;
  left: 50%;
  max-width: 560px;
  padding: 24px;
  position: fixed;
  top: 50%;
  transform: translate(-50%, -50%);
  width: min(560px, calc(100vw / var(--app-zoom, 1) - 48px));
}

.reader-no-shelf h2,
.reader-no-shelf p {
  margin: 0;
}

.reader-no-shelf .button {
  justify-self: start;
}
</style>
