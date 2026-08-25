<template>
  <div class="reader-app-empty">
    <p class="reader-app-empty-message">{{ t('readerApp.noBook') }}</p>
    <button v-if="canOpen" class="button" type="button" @click="onOpen">
      {{ t('readerApp.openBook') }}
    </button>
    <p v-if="failed" class="reader-app-empty-error" role="alert">
      {{ t('readerApp.openFailed') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from '@/i18n';
import { hasOpenBookBinding, openBookPackage } from './openBook';

// Where the reader app waits when the user cancelled the folder dialog. The
// app reloads the window itself once a book is open, so this view never has to
// route anywhere: it either stays, or it is replaced by the reload.
const { t } = useI18n();
const canOpen = hasOpenBookBinding();
const failed = ref(false);

async function onOpen(): Promise<void> {
  failed.value = false;
  try {
    await openBookPackage();
  } catch {
    failed.value = true;
  }
}
</script>

<style scoped>
.reader-app-empty {
  height: 100%;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 1rem;
  padding: 2rem;
  text-align: center;
}

.reader-app-empty-message {
  color: var(--text-muted, #6b6357);
  margin: 0;
}

.reader-app-empty-error {
  color: var(--danger, #b3261e);
  margin: 0;
}
</style>
