<template>
  <div class="mobile-connect">
    <section class="mobile-connect-card panel" role="form" :aria-label="t('mobileConnect.title')">
      <h1 class="mobile-connect-title">{{ t('mobileConnect.title') }}</h1>
      <p class="mobile-connect-description">{{ t('mobileConnect.description') }}</p>

      <div class="mobile-connect-modes" role="radiogroup" :aria-label="t('mobileConnect.modeLabel')">
        <button
          v-for="option in modeOptions"
          :key="option.value"
          type="button"
          class="button mobile-connect-mode"
          role="radio"
          :aria-checked="mode === option.value"
          :class="{ primary: mode === option.value }"
          @click="onModeSelect(option.value)"
        >
          {{ t(option.labelKey) }}
        </button>
      </div>

      <form class="mobile-connect-form" @submit.prevent="onSave">
        <template v-if="mode === 'server'">
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

        <p v-if="message" class="mobile-connect-error" role="alert">{{ message }}</p>

        <button type="submit" class="button primary" :disabled="saving || !canSave">
          {{ saving ? t('mobileConnect.saving') : t('mobileConnect.save') }}
        </button>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
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
import { useShelvesStore } from '@/composables/useShelvesStore';
import { buildAuthorizeUrl, generateRequestId, pollForToken } from '@/api/pcloud/auth';
import { PCloudClient } from '@/api/pcloud/client';
import { collectBookPackages, findBooksFolder } from '@/api/pcloud/bookpkg';
import {
  applyMobileConnectionConfig,
  loadMobileConnectionConfig,
  saveMobileConnectionConfig,
  type ConnectionMode,
  type MobileConnectionConfig
} from '@/providers/mobileConfig';

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

const modeOptions: Array<{ value: ConnectionMode; labelKey: string }> = [
  { value: 'server', labelKey: 'mobileConnect.modeServer' },
  { value: 'pcloud', labelKey: 'mobileConnect.modePCloud' }
];

const mode = ref<ConnectionMode>('server');
/** The mode as persisted when this page opened, so a switch can be detected. */
const savedMode = ref<ConnectionMode>('server');
const serverUrl = ref('');
const token = ref('');

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

const isAuthorized = computed(() => Boolean(pcloudAccessToken.value && pcloudHost.value));

// Surface a validation hint first, otherwise fall back to the shelf-store error.
// The shelf store only speaks for the server mode.
const message = computed(() => localError.value || (mode.value === 'server' ? shelvesError.value : ''));

const canSave = computed(() =>
  mode.value === 'server'
    ? selectedShelfID.value.length > 0
    : isAuthorized.value && pcloudShelfRoot.value.trim().length > 0 && pcloudBookCount.value !== null
);

onMounted(async () => {
  const config = await loadMobileConnectionConfig();
  mode.value = config.mode;
  savedMode.value = config.mode;
  serverUrl.value = config.serverUrl;
  token.value = config.token;
  pcloudClientId.value = config.pcloudClientId;
  pcloudAccessToken.value = config.pcloudAccessToken;
  pcloudHost.value = config.pcloudHost;
  pcloudShelfRoot.value = config.pcloudShelfRoot;

  if (config.mode === 'pcloud' && config.pcloudAccessToken && config.pcloudHost) {
    try {
      pcloudAccountIdentity.value = await loadPCloudAccountIdentity(
        config.pcloudHost,
        config.pcloudAccessToken
      );
    } catch (err) {
      localError.value = err instanceof Error ? err.message : String(err);
    }
  }

  // A returning user already has a server saved; load its shelves so the
  // dropdown is populated without an extra tap.
  if (config.mode === 'server' && config.serverUrl) {
    await loadShelvesForCurrentInput();
  }
});

function currentConfig(overrides: Partial<MobileConnectionConfig>): MobileConnectionConfig {
  return {
    mode: mode.value,
    serverUrl: serverUrl.value,
    token: token.value,
    shelfId: selectedShelfID.value,
    pcloudClientId: pcloudClientId.value,
    pcloudAccessToken: pcloudAccessToken.value,
    pcloudHost: pcloudHost.value,
    pcloudShelfRoot: pcloudShelfRoot.value,
    ...overrides
  };
}

function onModeSelect(next: ConnectionMode): void {
  mode.value = next;
  localError.value = '';
}

async function loadShelvesForCurrentInput(): Promise<void> {
  // Apply (without persisting) so the API client points at this server before
  // we list shelves; the values are only persisted on Save. Forced to server
  // mode so the pCloud short-circuit in listShelves cannot answer for it.
  await applyMobileConnectionConfig(currentConfig({ mode: 'server' }));
  // No persisted-shelf fallback here: a failed fetch for the server typed
  // above must leave Save disabled instead of reusing the previous server's
  // shelf id.
  await fetchShelves({ allowPersistedFallback: false });
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

/**
 * Restarts the app at the library, discarding everything the old connection
 * left in memory.
 *
 * Only a pCloud connection needs this. ServerBookshelfProvider re-reads the
 * module-level API base on every call, so pointing it at another server takes
 * effect on the next request — which is why saving has always been a plain
 * route change. PCloudBookshelfProvider instead captures its client and shelf
 * path at construction, and the provider is a singleton built once at
 * bootstrap; the shelf, layer and server-mode stores are module-level too.
 * Nothing short of a reload clears that.
 *
 * Loading a deep path directly is safe in the native shell: Capacitor serves
 * index.html for any path whose last segment has no extension (html5mode, on
 * by default — WebViewLocalServer.java). The query string is kept so the
 * browser preview stays in mobile mode, which a reload would otherwise lose
 * along with the latched runtime.
 */
function reloadIntoApp(): void {
  const url = new URL(window.location.href);
  url.pathname = '/books';
  url.hash = '';
  window.location.replace(url.toString());
}

async function onSave(): Promise<void> {
  localError.value = '';

  if (mode.value === 'server') {
    if (selectedShelfID.value.length === 0) {
      localError.value = t('mobileConnect.shelfRequired');
      return;
    }
  } else if (!canSave.value) {
    localError.value = t('mobileConnect.pcloud.verifyRequired');
    return;
  }

  saving.value = true;
  try {
    await saveMobileConnectionConfig(
      mode.value === 'server'
        ? { mode: 'server', serverUrl: serverUrl.value, token: token.value, shelfId: selectedShelfID.value }
        : {
            mode: 'pcloud',
            pcloudClientId: pcloudClientId.value,
            pcloudAccessToken: pcloudAccessToken.value,
            pcloudHost: pcloudHost.value,
            pcloudShelfRoot: pcloudShelfRoot.value.trim()
          }
    );
    if (mode.value === 'pcloud' || savedMode.value === 'pcloud') {
      reloadIntoApp();
    } else {
      await router.push('/books');
    }
  } catch (err) {
    localError.value = err instanceof Error ? err.message : String(err);
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
