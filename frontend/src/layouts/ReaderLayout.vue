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
import { computed, onMounted, ref } from 'vue';
import { useServerMode } from '@/composables/useServerMode';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useI18n } from '@/i18n';

const { t } = useI18n();
const { loading: shelvesLoading, loaded: shelvesLoaded, selectedShelfID, ensureShelvesLoaded } =
  useShelvesStore();
const { fetchServerMode } = useServerMode();

const booting = ref(true);

const hasActiveShelf = computed(
  () => !booting.value && shelvesLoaded.value && selectedShelfID.value.length > 0
);
const shelfUnavailableMessage = computed(() =>
  booting.value || shelvesLoading.value
    ? t('layout.shelf.loading')
    : t('layout.shelf.unavailableDescription')
);

onMounted(async () => {
  try {
    await Promise.all([fetchServerMode(), ensureShelvesLoaded()]);
  } finally {
    booting.value = false;
  }
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
