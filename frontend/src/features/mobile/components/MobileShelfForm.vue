<template>
  <section class="mobile-connect-card panel" role="form" :aria-label="heading">
    <h1 class="mobile-connect-title">{{ heading }}</h1>
    <p class="mobile-connect-description">{{ t('mobileConnect.description') }}</p>

    <div
      v-if="!entry"
      class="mobile-connect-modes"
      role="radiogroup"
      :aria-label="t('mobileConnect.modeLabel')"
    >
      <button
        v-for="option in typeOptions"
        :key="option.value"
        type="button"
        class="button mobile-connect-mode"
        role="radio"
        :aria-checked="type === option.value"
        :class="{ primary: type === option.value }"
        @click="onTypeSelect(option.value)"
      >
        {{ t(option.labelKey) }}
      </button>
    </div>

    <form class="mobile-connect-form" @submit.prevent="onSave">
      <label class="mobile-connect-field">
        <span class="mobile-connect-label">{{ t('mobileConnect.nameLabel') }}</span>
        <input
          v-model="name"
          class="input"
          type="text"
          :placeholder="t('mobileConnect.namePlaceholder')"
        />
        <span class="mobile-connect-hint">{{ t('mobileConnect.nameHint') }}</span>
      </label>

      <template v-if="type === 'server'">
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
            :model-value="shelfId"
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
      </template>

      <template v-else>
        <label class="mobile-connect-field">
          <span class="mobile-connect-label">{{ t('mobileConnect.pcloud.clientIdLabel') }}</span>
          <input
            v-model="pcloudClientId"
            class="input"
            type="text"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileConnect.pcloud.clientIdPlaceholder')"
          />
          <span class="mobile-connect-hint">{{ t('mobileConnect.pcloud.clientIdHint') }}</span>
        </label>

        <button
          type="button"
          class="button"
          :disabled="authorizing || pcloudClientId.trim().length === 0"
          @click="onAuthorize"
        >
          {{ authorizing ? t('mobileConnect.pcloud.authorizing') : t('mobileConnect.pcloud.authorize') }}
        </button>

        <button v-if="authorizing" type="button" class="button" @click="onCancelAuthorize">
          {{ t('mobileConnect.pcloud.cancel') }}
        </button>

        <p v-if="isAuthorized" class="mobile-connect-hint" role="status">
          {{
            pcloudAccountIdentity
              ? t('mobileConnect.pcloud.authorizedAccount', {
                  account: pcloudAccountIdentity,
                  host: pcloudHost
                })
              : t('mobileConnect.pcloud.authorized', { host: pcloudHost })
          }}
        </p>

        <label class="mobile-connect-field">
          <span class="mobile-connect-label">{{ t('mobileConnect.pcloud.shelfRootLabel') }}</span>
          <input
            v-model="pcloudShelfRoot"
            class="input"
            type="text"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileConnect.pcloud.shelfRootPlaceholder')"
            @input="pcloudBookCount = null"
          />
          <span class="mobile-connect-hint">{{ t('mobileConnect.pcloud.shelfRootHint') }}</span>
        </label>

        <button
          type="button"
          class="button"
          :disabled="verifying || !isAuthorized || pcloudShelfRoot.trim().length === 0"
          @click="onVerifyPCloudShelf"
        >
          {{ verifying ? t('mobileConnect.pcloud.verifying') : t('mobileConnect.pcloud.verify') }}
        </button>

        <p v-if="pcloudBookCount !== null" class="mobile-connect-hint" role="status">
          {{ t('mobileConnect.pcloud.shelfFound', { count: String(pcloudBookCount) }) }}
        </p>
      </template>

      <p v-if="entry" class="mobile-connect-hint">{{ t('mobileConnect.retargetHint') }}</p>

      <p v-if="message" class="mobile-connect-error" role="alert">{{ message }}</p>

      <button type="submit" class="button primary" :disabled="saving || !canSave">
        {{ saving ? t('mobileConnect.saving') : t('mobileConnect.save') }}
      </button>

      <button v-if="cancellable" type="button" class="button" @click="emit('cancel')">
        {{ t('mobileConnect.cancel') }}
      </button>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { Browser } from '@capacitor/browser';
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
import { useI18n } from '@/i18n';
import { setApiBase } from '@/api/client';
import { listServerShelves, type ShelfInfo } from '@/api/shelves';
import { buildAuthorizeUrl, generateRequestId, pollForToken } from '@/api/pcloud/auth';
import { PCloudClient } from '@/api/pcloud/client';
import { collectBookPackages, findBooksFolder } from '@/api/pcloud/bookpkg';
import {
  applyActiveShelfEntry,
  getShelfEntryToken,
  newShelfEntry,
  upsertShelfEntry,
  type ShelfEntry,
  type ShelfSourceType
} from '@/providers/mobileConfig';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';

const props = defineProps<{
  /** The entry being edited, or null when adding one. */
  entry?: ShelfEntry | null;
  cancellable?: boolean;
}>();

const emit = defineEmits<{ (event: 'cancel'): void }>();

const { t } = useI18n();

const entry = computed(() => props.entry ?? null);

const typeOptions: Array<{ value: ShelfSourceType; labelKey: string }> = [
  { value: 'server', labelKey: 'mobileConnect.modeServer' },
  { value: 'pcloud', labelKey: 'mobileConnect.modePCloud' }
];

const type = ref<ShelfSourceType>('server');
const name = ref('');
const serverUrl = ref('');
const token = ref('');
const shelfId = ref('');
const shelves = ref<ShelfInfo[]>([]);
const shelvesLoading = ref(false);

const pcloudClientId = ref('');
const pcloudAccessToken = ref('');
const pcloudHost = ref('');
const pcloudShelfRoot = ref('');
/** Email (or userid fallback) returned by pCloud userinfo for the active token. */
const pcloudAccountIdentity = ref('');
/** Set by a successful shelf check; null means "not verified yet". */
const pcloudBookCount = ref<number | null>(null);

const saving = ref(false);
const authorizing = ref(false);
const verifying = ref(false);
const localError = ref('');

let authAbort: AbortController | null = null;

const heading = computed(() =>
  entry.value ? t('mobileConnect.editTitle') : t('mobileConnect.title')
);
const isAuthorized = computed(() => Boolean(pcloudAccessToken.value && pcloudHost.value));
const message = computed(() => localError.value);

const canSave = computed(() =>
  type.value === 'server'
    ? serverUrl.value.trim().length > 0 && shelfId.value.length > 0
    : isAuthorized.value && pcloudShelfRoot.value.trim().length > 0 && pcloudBookCount.value !== null
);

onMounted(async () => {
  const existing = entry.value;
  if (!existing) {
    return;
  }

  type.value = existing.type;
  name.value = existing.name;
  const secret = await getShelfEntryToken(existing.id);

  if (existing.type === 'pcloud') {
    pcloudClientId.value = existing.pcloudClientId;
    pcloudHost.value = existing.pcloudHost;
    pcloudShelfRoot.value = existing.pcloudShelfRoot;
    pcloudAccessToken.value = secret;
    if (secret && existing.pcloudHost) {
      try {
        pcloudAccountIdentity.value = await loadPCloudAccountIdentity(existing.pcloudHost, secret);
      } catch (err) {
        localError.value = err instanceof Error ? err.message : String(err);
      }
    }
    return;
  }

  serverUrl.value = existing.serverUrl;
  shelfId.value = existing.shelfId;
  token.value = secret;
  // A returning user already has a server saved; load its shelves so the
  // dropdown is populated without an extra tap.
  if (existing.serverUrl) {
    await loadShelves();
  }
});

function onTypeSelect(next: ShelfSourceType): void {
  type.value = next;
  localError.value = '';
}

/**
 * Points the API client at the server currently typed into the form, runs
 * `probe`, and puts the active shelf back.
 *
 * The client's base URL and token hook are module-level state shared with the
 * running app, and the base URL is half of the device-local cache scope key
 * (providers/cacheScope.ts). Leaving a draft applied would attribute another
 * shelf's downloads to whatever was typed here, so the restore is in `finally`
 * rather than on the success path.
 */
async function withServerProbe<T>(probe: () => Promise<T>): Promise<T> {
  const probeToken = token.value;
  setApiBase(serverUrl.value);
  window.plainshelf = { getApiToken: async () => probeToken };
  try {
    return await probe();
  } finally {
    await applyActiveShelfEntry();
  }
}

async function loadShelves(): Promise<void> {
  shelvesLoading.value = true;
  try {
    const loaded = await withServerProbe(listServerShelves);
    shelves.value = loaded;
    // A shelf the typed server does not offer cannot stay selected, or Save
    // would write an entry pointing at nothing.
    if (!loaded.some((shelf) => shelf.id === shelfId.value)) {
      shelfId.value = '';
    }
  } catch (err) {
    shelves.value = [];
    shelfId.value = '';
    localError.value = err instanceof Error ? err.message : String(err);
  } finally {
    shelvesLoading.value = false;
  }
}

async function onLoadShelves(): Promise<void> {
  localError.value = '';
  if (serverUrl.value.trim().length === 0) {
    localError.value = t('mobileConnect.serverUrlRequired');
    return;
  }
  await loadShelves();
}

function onShelfSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    shelfId.value = value;
  }
}

/** Verifies that a token and regional host resolve to a real pCloud account. */
async function loadPCloudAccountIdentity(host: string, accessToken: string): Promise<string> {
  const info = await new PCloudClient({ host, accessToken }).getUserInfo();
  return info.email?.trim() || String(info.userid);
}

/**
 * Runs pCloud's poll_token flow: open the approval page in the system browser
 * and wait for the token. The region comes back with it, so nothing here has to
 * be told which regional host serves the account.
 */
async function onAuthorize(): Promise<void> {
  localError.value = '';
  const clientId = pcloudClientId.value.trim();
  if (!clientId) {
    localError.value = t('mobileConnect.pcloud.clientIdRequired');
    return;
  }

  authorizing.value = true;
  authAbort = new AbortController();
  try {
    const requestId = generateRequestId();
    await Browser.open({ url: buildAuthorizeUrl(clientId, requestId) });
    const session = await pollForToken({ clientId, requestId, signal: authAbort.signal });
    const accountIdentity = await loadPCloudAccountIdentity(session.host, session.accessToken);

    pcloudAccessToken.value = session.accessToken;
    pcloudHost.value = session.host;
    pcloudAccountIdentity.value = accountIdentity;
    // New credentials may reach a different account, so the previous shelf
    // check no longer vouches for anything.
    pcloudBookCount.value = null;
    await closeBrowser();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : String(err);
  } finally {
    authorizing.value = false;
    authAbort = null;
  }
}

function onCancelAuthorize(): void {
  authAbort?.abort();
}

async function closeBrowser(): Promise<void> {
  // Closing is best-effort: the web implementation opens a tab it cannot close,
  // and a browser the user already dismissed is not an error worth surfacing.
  try {
    await Browser.close();
  } catch {
    // ignored
  }
}

/**
 * The pCloud equivalent of loading a server's shelves: prove the credentials
 * work and that the named folder really is a shelf, before anything is saved.
 */
async function onVerifyPCloudShelf(): Promise<void> {
  localError.value = '';
  pcloudBookCount.value = null;
  const shelfRoot = pcloudShelfRoot.value.trim();
  if (!shelfRoot) {
    localError.value = t('mobileConnect.pcloud.shelfRootRequired');
    return;
  }

  verifying.value = true;
  try {
    const client = new PCloudClient({
      host: pcloudHost.value,
      accessToken: pcloudAccessToken.value
    });
    const booksFolder = findBooksFolder(await client.listFolderRecursive({ path: shelfRoot }));
    if (!booksFolder) {
      localError.value = t('mobileConnect.pcloud.notAShelf');
      return;
    }
    pcloudBookCount.value = collectBookPackages(booksFolder).length;
  } catch (err) {
    localError.value = err instanceof Error ? err.message : String(err);
  } finally {
    verifying.value = false;
  }
}

function draftEntry(): { entry: ShelfEntry; secret: string } {
  // A new entry mints its id here; an edited one keeps its own, so the shelf it
  // names stays the same shelf to the picker and to the active-entry pointer.
  const base = entry.value ?? newShelfEntry(type.value);
  if (type.value === 'pcloud') {
    return {
      entry: {
        id: base.id,
        type: 'pcloud',
        name: name.value.trim(),
        pcloudClientId: pcloudClientId.value.trim(),
        pcloudHost: pcloudHost.value,
        pcloudShelfRoot: pcloudShelfRoot.value.trim()
      },
      secret: pcloudAccessToken.value
    };
  }

  return {
    entry: {
      id: base.id,
      type: 'server',
      name: name.value.trim(),
      serverUrl: serverUrl.value,
      shelfId: shelfId.value
    },
    secret: token.value
  };
}

async function onSave(): Promise<void> {
  localError.value = '';

  if (type.value === 'server') {
    if (shelfId.value.length === 0) {
      localError.value = t('mobileConnect.shelfRequired');
      return;
    }
  } else if (!canSave.value) {
    localError.value = t('mobileConnect.pcloud.verifyRequired');
    return;
  }

  saving.value = true;
  try {
    const draft = draftEntry();
    await upsertShelfEntry(draft.entry, draft.secret);
    reloadIntoApp();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : String(err);
    saving.value = false;
  }
}
</script>

<style scoped>
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

.mobile-connect-modes {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

.mobile-connect-mode {
  flex: 1;
  justify-content: center;
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
