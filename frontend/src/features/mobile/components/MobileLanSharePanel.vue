<template>
  <section class="mobile-lan-share" :aria-label="t('mobileShelves.lanShare.title')">
    <h2 class="mobile-lan-share-title">{{ t('mobileShelves.lanShare.title') }}</h2>
    <p class="mobile-lan-share-description">
      {{ t('mobileShelves.lanShare.description', { minutes: status?.idleLimitMinutes ?? 30 }) }}
    </p>

    <fieldset class="mobile-lan-share-shelves" :disabled="busy">
      <legend>{{ t('mobileShelves.lanShare.shelvesLabel') }}</legend>
      <label v-for="shelf in shelves" :key="shelf.id" class="mobile-lan-share-shelf">
        <input v-model="selected" type="checkbox" :value="shelf.id" @change="onSelectionChange" />
        {{ shelf.name }}
      </label>
    </fieldset>

    <dl v-if="status?.running" class="mobile-lan-share-info">
      <dt>{{ t('mobileShelves.lanShare.urlLabel') }}</dt>
      <dd class="mobile-lan-share-url">{{ status.url }}</dd>
      <dt>{{ t('mobileShelves.lanShare.tokenLabel') }}</dt>
      <dd class="mobile-lan-share-token">{{ status.token }}</dd>
    </dl>
    <img
      v-if="status?.running && status.qr_code"
      class="mobile-lan-share-qr"
      :src="status.qr_code"
      :alt="t('settings.lanShare.qrAlt')"
    />
    <p v-if="status?.running && status.error" class="mobile-lan-share-error" role="alert">{{ status.error }}</p>
    <p v-if="!status?.running && status?.endedReason" class="mobile-lan-share-ended" role="status">
      {{ t('mobileShelves.lanShare.ended', { reason: status.endedReason }) }}
    </p>
    <p v-if="error" class="mobile-lan-share-error" role="alert">{{ error }}</p>

    <div class="mobile-lan-share-actions">
      <button v-if="status?.running" type="button" class="button mobile-lan-share-rotate" :disabled="busy" @click="onRotate">
        {{ t('settings.lanShare.rotate') }}
      </button>
      <button
        v-if="status?.running"
        type="button"
        class="button mobile-lan-share-stop"
        :disabled="busy"
        @click="onStop"
      >
        {{ t('mobileShelves.lanShare.stop') }}
      </button>
      <button
        v-else
        type="button"
        class="button mobile-lan-share-start"
        :disabled="busy || selected.length === 0"
        @click="onStart"
      >
        {{ t('mobileShelves.lanShare.start') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import type { PluginListenerHandle } from '@capacitor/core';
import { useI18n } from '@/i18n';
import { LanShare, type LanShareStatus } from '@/features/mobile/utils/lanShare';

const props = defineProps<{
  /** The device's local shelves; only these can be shared. */
  shelves: Array<{ id: string; name: string }>;
}>();

const { t } = useI18n();
const status = ref<LanShareStatus | null>(null);
/** Nothing is ticked until the user ticks it, as on the desktop. */
const selected = ref<string[]>([]);
const busy = ref(false);
const error = ref('');
let handle: PluginListenerHandle | null = null;

function apply(next: LanShareStatus): void {
  status.value = next;
  if (next.running) {
    selected.value = next.shelf_ids.filter((id) => props.shelves.some((shelf) => shelf.id === id));
  }
}

/**
 * A removed shelf leaves the share in Go but not this selection; pruned here,
 * or the next start would name a shelf that no longer exists.
 */
watch(
  () => props.shelves,
  async (shelves) => {
    const kept = selected.value.filter((id) => shelves.some((shelf) => shelf.id === id));
    if (kept.length === selected.value.length) {
      return;
    }
    selected.value = kept;
    await onSelectionChange();
  }
);

onMounted(async () => {
  try {
    apply(await LanShare.getStatus());
    handle = await LanShare.addListener('lanShareChanged', apply);
  } catch (err) {
    error.value = explain(err);
  }
});

onBeforeUnmount(() => {
  void handle?.remove();
});

function explain(err: unknown): string {
  const message = err instanceof Error ? err.message : String(err);
  return message.includes('not connected to Wi-Fi') ? t('mobileShelves.lanShare.noWifi') : message;
}

async function run(action: () => Promise<LanShareStatus>): Promise<void> {
  busy.value = true;
  error.value = '';
  try {
    apply(await action());
  } catch (err) {
    error.value = explain(err);
  } finally {
    busy.value = false;
  }
}

function onStart(): Promise<void> {
  return run(() => LanShare.start({ shelfIds: selected.value }));
}

function onRotate(): Promise<void> {
  return run(() => LanShare.rotateToken());
}

function onStop(): Promise<void> {
  return run(() => LanShare.stop());
}

/** While running, a changed tick applies at once; unticking the last one stops. */
function onSelectionChange(): Promise<void> | undefined {
  if (!status.value?.running) {
    return undefined;
  }
  return selected.value.length === 0 ? onStop() : onStart();
}
</script>

<style scoped>
.mobile-lan-share {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--border, #d9d2c4);
  border-radius: 8px;
}

.mobile-lan-share-title {
  margin: 0;
  font-size: 1rem;
}

.mobile-lan-share-description,
.mobile-lan-share-ended {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted, #666);
}

.mobile-lan-share-shelves {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 0;
  border: 0;
}

.mobile-lan-share-shelves legend {
  padding: 0;
  margin-bottom: 4px;
  font-size: 0.85rem;
  font-weight: 600;
}

.mobile-lan-share-info {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 8px;
  margin: 0;
  font-size: 0.85rem;
}

.mobile-lan-share-info dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-family: var(--font-mono, monospace);
  user-select: all;
}

.mobile-lan-share-error {
  margin: 0;
  color: var(--danger, #b3261e);
  font-size: 0.9rem;
}

.mobile-lan-share-qr {
  width: 100%;
  max-width: 320px;
  image-rendering: pixelated;
  justify-self: center;
}

.mobile-lan-share-actions {
  flex-wrap: wrap;
  display: flex;
  gap: 8px;
}
</style>
