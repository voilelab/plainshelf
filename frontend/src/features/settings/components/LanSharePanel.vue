<template>
  <section class="panel settings-group" data-testid="lan-share-panel">
    <h3>{{ t('settings.lanShare.title') }}</h3>

    <label class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.lanShare.label') }}</div>
        <p class="setting-description">
          {{ t('settings.lanShare.description') }}
        </p>
      </div>
      <input
        class="setting-checkbox"
        type="checkbox"
        :checked="status?.enabled ?? false"
        :disabled="busy || !status"
        data-testid="lan-share-enabled"
        @change="onToggle($event)"
      />
    </label>

    <p v-if="error" class="settings-message settings-message-error" role="alert">
      {{ error }}
    </p>

    <template v-if="status?.enabled">
      <p v-if="status.addresses.length === 0" class="setting-description">
        {{ t('settings.lanShare.noAddress') }}
      </p>
      <!-- A computer on two networks picks which one the export binds; never all. -->
      <div v-else-if="status.addresses.length > 1" class="setting-item">
        <div class="setting-label">
          {{ t('settings.lanShare.addressLabel') }}
        </div>
        <SelectRoot :model-value="status.host" :disabled="busy" @update:model-value="onHost">
          <SelectTrigger class="setting-select select-trigger" data-testid="lan-share-host">
            <SelectValue>{{ status.host }}</SelectValue>
          </SelectTrigger>
          <SelectPortal>
            <SelectContent class="reka-menu" position="popper" align="end" :side-offset="6">
              <SelectViewport>
                <SelectItem
                  v-for="address in status.addresses"
                  :key="address"
                  class="reka-menu-item"
                  :value="address"
                >
                  <SelectItemText>{{ address }}</SelectItemText>
                </SelectItem>
              </SelectViewport>
            </SelectContent>
          </SelectPortal>
        </SelectRoot>
      </div>

      <fieldset class="lan-share-shelves">
        <legend class="setting-label">
          {{ t('settings.lanShare.shelvesLabel') }}
        </legend>
        <label v-for="shelf in status.shelves" :key="shelf.id" class="lan-share-shelf">
          <input
            type="checkbox"
            :checked="shelf.shared"
            :disabled="busy"
            :data-testid="`lan-share-shelf-${shelf.id}`"
            @change="onShelf(shelf.id, $event)"
          />
          <span>{{ shelf.name }}</span>
          <span class="setting-value">{{ shelf.id }}</span>
        </label>
        <p v-if="sharedCount === 0" class="setting-description">
          {{ t('settings.lanShare.noneShared') }}
        </p>
      </fieldset>

      <div v-if="status.running" class="lan-share-connect" data-testid="lan-share-connect">
        <img
          v-if="status.qr_code"
          class="lan-share-qr"
          :src="status.qr_code"
          :alt="t('settings.lanShare.qrAlt')"
          width="180"
          height="180"
        />
        <dl class="lan-share-details">
          <dt class="setting-label">{{ t('settings.lanShare.urlLabel') }}</dt>
          <dd class="setting-value" data-testid="lan-share-url">
            {{ status.url }}
          </dd>
          <dt class="setting-label">{{ t('settings.lanShare.tokenLabel') }}</dt>
          <dd class="setting-value lan-share-token" data-testid="lan-share-token">
            {{ status.token }}
          </dd>
          <dd>
            <button class="button" type="button" :disabled="busy" @click="onRotate">
              {{ t('settings.lanShare.rotate') }}
            </button>
          </dd>
        </dl>
      </div>

      <p class="setting-description">{{ t('settings.lanShare.firewall') }}</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
import {
  getDesktopLANShare,
  rotateDesktopLANShareToken,
  setDesktopLANShare,
  type DesktopLANShareStatus
} from '@/api/desktop';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const status = ref<DesktopLANShareStatus | null>(null);
const busy = ref(false);
const error = ref('');

const sharedCount = computed(() => status.value?.shelves.filter((s) => s.shared).length ?? 0);

// Wails rejects with the Go error's message string, not an Error.
function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

async function run(action: () => Promise<DesktopLANShareStatus>): Promise<void> {
  busy.value = true;
  error.value = '';
  try {
    status.value = await action();
    error.value = status.value.error;
  } catch (err) {
    error.value = message(err);
    // The switch may have been turned back off; show what Go now holds.
    try {
      status.value = await getDesktopLANShare();
    } catch {
      // keep the previous state
    }
  } finally {
    busy.value = false;
  }
}

function submit(change: { enabled?: boolean; host?: string; shelfIDs?: string[] }): Promise<void> {
  const current = status.value;
  if (!current) return Promise.resolve();
  return run(() =>
    setDesktopLANShare({
      enabled: change.enabled ?? current.enabled,
      host: change.host ?? current.host,
      shelfIDs: change.shelfIDs ?? current.shelves.filter((s) => s.shared).map((s) => s.id)
    })
  );
}

// A native checkbox flips itself on click; when Go refuses, the bound value
// does not change, so Vue would not flip it back.
function checkbox(event: Event): HTMLInputElement {
  return event.target as HTMLInputElement;
}

async function onToggle(event: Event): Promise<void> {
  const el = checkbox(event);
  await submit({ enabled: el.checked });
  el.checked = status.value?.enabled ?? false;
}

function onHost(value: AcceptableValue): void {
  if (typeof value === 'string') void submit({ host: value });
}

async function onShelf(id: string, event: Event): Promise<void> {
  const el = checkbox(event);
  const current = status.value?.shelves.filter((s) => s.shared).map((s) => s.id) ?? [];
  await submit({
    shelfIDs: el.checked ? [...current, id] : current.filter((x) => x !== id)
  });
  el.checked = status.value?.shelves.find((s) => s.id === id)?.shared ?? false;
}

function onRotate(): void {
  void run(rotateDesktopLANShareToken);
}

onMounted(() => {
  void run(getDesktopLANShare);
});
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.lan-share-shelves {
  border: none;
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
}

.lan-share-shelf {
  align-items: center;
  display: flex;
  gap: 8px;
}

.lan-share-connect {
  align-items: flex-start;
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.lan-share-qr {
  /* Crisp modules: the PNG is small and scaled up. */
  image-rendering: pixelated;
}

.lan-share-details {
  display: grid;
  gap: 4px;
  margin: 0;
  min-width: 0;
}

.lan-share-details dd {
  margin: 0 0 8px;
}

.lan-share-token {
  overflow-wrap: anywhere;
  user-select: all;
}
</style>
