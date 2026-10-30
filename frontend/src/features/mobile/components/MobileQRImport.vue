<template>
  <div class="mobile-qr-import">
    <button type="button" class="button" :disabled="busy" @click="scan">
      {{ t('mobileQR.scan') }}
    </button>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="preview">
      <p>{{ t('settings.shelves.qrHelp') }}</p>
      <ul>
        <li v-for="shelf in preview.shelves" :key="shelf.id">{{ shelf.name }} — {{ shelf.url }}</li>
      </ul>
      <button type="button" class="button primary" :disabled="busy || !preview.shelves.length" @click="confirm">
        {{ t('settings.shelves.qrConfirm') }}
      </button>
      <button type="button" class="button" :disabled="busy" @click="clear">{{ t('mobileConnect.cancel') }}</button>
    </template>
    <ul v-if="results.length" aria-live="polite">
      <li v-for="(result, index) in results" :key="index">{{ result.name }} — {{ t(result.message) }}</li>
    </ul>
    <button v-if="added" type="button" class="button primary" :disabled="busy" @click="reloadIntoApp()">
      {{ t('mobileQR.open') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue';
import { Capacitor, registerPlugin } from '@capacitor/core';
import { useI18n } from '@/i18n';
import { LocalShelf } from '@/providers/localShelf';
import { getShelfEntries, newShelfEntry, upsertShelfEntry } from '@/providers/mobileConfig';
import { shelfSourceCheckMessageKey } from '@/utils/shelfSourceCheck';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';

const Scanner = registerPlugin<{
  scan(): Promise<{ cancelled: boolean; text?: string }>;
}>('PlainShelfQRScanner');
const emit = defineEmits<{ (event: 'busy', value: boolean): void }>();
const { t } = useI18n();
const preview = shallowRef<Awaited<ReturnType<typeof LocalShelf.parseLANShareQR>> | null>(null);
const busy = ref(false);
watch(busy, (value) => emit('busy', value), { flush: 'sync' });
const error = ref('');
const results = ref<Array<{ name: string; message: string }>>([]);
const added = ref(false);
let alive = true;
onBeforeUnmount(() => {
  alive = false;
  emit('busy', false);
  clear();
});

function clear(): void {
  if (preview.value) preview.value.token = '';
  preview.value = null;
  error.value = '';
}

async function scan(): Promise<void> {
  if (busy.value) return;
  clear();
  results.value = [];
  busy.value = true;
  try {
    if (!Capacitor.isNativePlatform() || !Capacitor.isPluginAvailable('PlainShelfQRScanner')) {
      error.value = t('mobileQR.unavailable');
      return;
    }
    const result = await Scanner.scan();
    if (!alive || result.cancelled) return;
    if (!result.text) {
      error.value = t('mobileQR.failed');
      return;
    }
    try {
      const parsed = await LocalShelf.parseLANShareQR({ text: result.text });
      if (alive) preview.value = parsed;
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '';
      error.value = message || t('mobileQR.invalid');
    }
  } catch {
    error.value = t('mobileQR.failed');
  } finally {
    busy.value = false;
  }
}

async function confirm(): Promise<void> {
  if (busy.value || !preview.value) return;
  const payload = preview.value;
  preview.value = null;
  results.value = [];
  busy.value = true;
  try {
    for (const shelf of payload.shelves) {
      if (!alive) break;
      let message = 'settings.shelves.qrFailed';
      const duplicate = getShelfEntries().some((entry) => entry.type === 'dav' &&
        entry.davSource === 'plainshelf' && entry.davUrl.replace(/\/+$/, '') === shelf.url.replace(/\/+$/, ''));
      if (duplicate) {
        message = 'settings.shelves.qrExists';
      } else {
        try {
          const check = await LocalShelf.checkSource({ source: 'plainshelf', url: shelf.url, username: '', secret: payload.token });
          if (!alive) break;
          const failure = shelfSourceCheckMessageKey(check);
          if (failure) {
            message = failure;
          } else {
            const base = newShelfEntry('dav');
            await upsertShelfEntry({ id: base.id, type: 'dav', name: shelf.name, davSource: 'plainshelf', davUrl: shelf.url }, payload.token);
            added.value = true;
            message = 'settings.shelves.qrAdded';
          }
        } catch {
        }
      }
      results.value.push({ name: shelf.name, message });
    }
  } finally {
    payload.token = '';
    busy.value = false;
  }
}
</script>

<style scoped>
.mobile-qr-import { display: grid; gap: 8px; }
ul { padding-inline-start: 20px; overflow-wrap: anywhere; }
</style>
