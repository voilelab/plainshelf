<template>
  <div class="mobile-connect">
    <section class="mobile-connect-card panel" role="form" :aria-label="t('mobileConnect.title')">
      <h1 class="mobile-connect-title">{{ t('mobileConnect.title') }}</h1>
      <p class="mobile-connect-description">{{ t('mobileConnect.description') }}</p>

      <form class="mobile-connect-form" @submit.prevent="onSave">
        <label class="mobile-connect-field">
          <span class="mobile-connect-label">{{ t('mobileConnect.serverUrlLabel') }}</span>
          <input
            v-model="serverUrl"
            class="input"
            type="url"
            inputmode="url"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileConnect.serverUrlPlaceholder')"
          />
        </label>

        <label class="mobile-connect-field">
          <span class="mobile-connect-label">{{ t('mobileConnect.tokenLabel') }}</span>
          <input
            v-model="token"
            class="input"
            type="password"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileConnect.tokenPlaceholder')"
          />
          <span class="mobile-connect-hint">{{ t('mobileConnect.tokenHint') }}</span>
        </label>

        <button
          type="button"
          class="button"
          :disabled="shelvesLoading || serverUrl.trim().length === 0"
          @click="onLoadShelves"
        >
          {{ shelvesLoading ? t('mobileConnect.loadingShelves') : t('mobileConnect.loadShelves') }}
        </button>

        <label class="mobile-connect-field">
          <span class="mobile-connect-label">{{ t('mobileConnect.shelfLabel') }}</span>
          <SelectRoot
            :model-value="selectedShelfID"
            :disabled="shelvesLoading || shelves.length === 0"
            @update:model-value="onShelfSelect"
          >
            <SelectTrigger class="button mobile-connect-shelf-select">
              <SelectValue :placeholder="t('mobileConnect.shelfPlaceholder')" />
            </SelectTrigger>
            <SelectPortal>
              <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                <SelectViewport>
                  <SelectItem
                    v-for="shelf in shelves"
                    :key="shelf.id"
                    class="reka-menu-item"
                    :value="shelf.id"
                  >
                    <SelectItemText>{{ shelf.name }}</SelectItemText>
                  </SelectItem>
                </SelectViewport>
              </SelectContent>
            </SelectPortal>
          </SelectRoot>
        </label>

        <p v-if="message" class="mobile-connect-error" role="alert">{{ message }}</p>

        <button
          type="submit"
          class="button primary"
          :disabled="saving || selectedShelfID.length === 0"
        >
          {{ saving ? t('mobileConnect.saving') : t('mobileConnect.save') }}
        </button>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
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
import { useI18n } from '../i18n';
import { useShelvesStore } from '../composables/useShelvesStore';
import {
  applyMobileConnectionConfig,
  loadMobileConnectionConfig,
  saveMobileConnectionConfig
} from '../providers/mobileConfig';

const { t } = useI18n();
const router = useRouter();
const {
  shelves,
  selectedShelfID,
  loading: shelvesLoading,
  error: shelvesError,
  fetchShelves,
  selectShelf
} = useShelvesStore();

const serverUrl = ref('');
const token = ref('');
const saving = ref(false);
const localError = ref('');

// Surface a validation hint first, otherwise fall back to the shelf-store error.
const message = computed(() => localError.value || shelvesError.value);

onMounted(async () => {
  const config = await loadMobileConnectionConfig();
  serverUrl.value = config.serverUrl;
  token.value = config.token;

  // A returning user already has a server saved; load its shelves so the
  // dropdown is populated without an extra tap.
  if (config.serverUrl) {
    await loadShelvesForCurrentInput();
  }
});

async function loadShelvesForCurrentInput(): Promise<void> {
  // Apply (without persisting) so the API client points at this server before
  // we list shelves; the values are only persisted on Save.
  await applyMobileConnectionConfig({ serverUrl: serverUrl.value, token: token.value });
  await fetchShelves();
}

async function onLoadShelves(): Promise<void> {
  localError.value = '';
  if (serverUrl.value.trim().length === 0) {
    localError.value = t('mobileConnect.serverUrlRequired');
    return;
  }
  await loadShelvesForCurrentInput();
}

function onShelfSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    selectShelf(value);
  }
}

async function onSave(): Promise<void> {
  localError.value = '';
  if (selectedShelfID.value.length === 0) {
    localError.value = t('mobileConnect.shelfRequired');
    return;
  }

  saving.value = true;
  try {
    await saveMobileConnectionConfig({
      serverUrl: serverUrl.value,
      token: token.value,
      shelfId: selectedShelfID.value
    });
    await router.push('/books');
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.mobile-connect {
  min-height: calc(100vh / var(--app-zoom, 1));
  width: 100%;
  box-sizing: border-box;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 32px 16px;
  overflow-y: auto;
  background:
    radial-gradient(circle at top, rgba(255, 250, 240, 0.92), rgba(245, 241, 232, 0.96) 42%),
    linear-gradient(180deg, #f7f2e7 0%, #efe7d7 100%);
}

.mobile-connect-card {
  width: min(480px, 100%);
  padding: 24px;
  display: grid;
  gap: 12px;
}

.mobile-connect-title {
  margin: 0;
  font-size: 1.4rem;
}

.mobile-connect-description {
  margin: 0;
  color: var(--text-muted, #666);
}

.mobile-connect-form {
  display: grid;
  gap: 16px;
  margin-top: 8px;
}

.mobile-connect-field {
  display: grid;
  gap: 6px;
}

.mobile-connect-label {
  font-weight: 600;
  font-size: 0.9rem;
}

.mobile-connect-hint {
  font-size: 0.8rem;
  color: var(--text-muted, #666);
}

.mobile-connect-shelf-select {
  width: 100%;
  justify-content: space-between;
}

.mobile-connect-error {
  margin: 0;
  color: var(--danger, #b3261e);
  font-size: 0.9rem;
}
</style>
