<template>
  <section class="mobile-shelf-form-card panel" role="form" :aria-label="heading">
    <h1 class="mobile-shelf-form-title">{{ heading }}</h1>
    <p class="mobile-shelf-form-description">{{ t('mobileShelfForm.description') }}</p>

    <ShelfSourceModes
      v-if="!entry"
      :model-value="type"
      :options="typeOptions"
      :label="t('mobileShelfForm.modeLabel')"
      :disabled="saving || qrBusy"
      @update:model-value="onTypeSelect"
    />

    <MobileQRImport v-if="type === 'qr'" @busy="qrBusy = $event" />

    <form v-else class="mobile-shelf-form-form" @submit.prevent="onSave">
      <fieldset class="mobile-shelf-form-fields" :disabled="saving">
      <label class="mobile-shelf-form-field">
        <span class="mobile-shelf-form-label">{{ t('mobileShelfForm.nameLabel') }}</span>
        <input
          v-model="name"
          class="input"
          type="text"
          :placeholder="t('mobileShelfForm.namePlaceholder')"
        />
        <span class="mobile-shelf-form-hint">{{ t('mobileShelfForm.nameHint') }}</span>
      </label>

      <template v-if="type === 'server'">
        <label class="mobile-shelf-form-field">
          <span class="mobile-shelf-form-label">{{ t('mobileShelfForm.serverUrlLabel') }}</span>
          <input
            v-model="serverUrl"
            class="input"
            type="url"
            inputmode="url"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileShelfForm.serverUrlPlaceholder')"
          />
        </label>

        <label class="mobile-shelf-form-field">
          <span class="mobile-shelf-form-label">{{ t('mobileShelfForm.tokenLabel') }}</span>
          <input
            v-model="token"
            class="input"
            type="password"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileShelfForm.tokenPlaceholder')"
          />
          <span class="mobile-shelf-form-hint">{{ t('mobileShelfForm.tokenHint') }}</span>
        </label>

        <button
          type="button"
          class="button"
          :disabled="shelvesLoading || serverUrl.trim().length === 0"
          @click="onLoadShelves"
        >
          {{ shelvesLoading ? t('mobileShelfForm.loadingShelves') : t('mobileShelfForm.loadShelves') }}
        </button>

        <label class="mobile-shelf-form-field">
          <span class="mobile-shelf-form-label">{{ t('mobileShelfForm.shelfLabel') }}</span>
          <SelectRoot
            :model-value="shelfId"
            :disabled="shelvesLoading || shelves.length === 0"
            @update:model-value="onShelfSelect"
          >
            <SelectTrigger class="button mobile-shelf-form-shelf-select">
              <SelectValue :placeholder="t('mobileShelfForm.shelfPlaceholder')" />
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

      <p v-else-if="type === 'local'" class="mobile-shelf-form-hint">{{ t('mobileShelfForm.localHint') }}</p>

      <template v-else-if="type === 'dav'">
        <RemoteSourceFields
          v-model:kind="davSource"
          v-model:url="davUrl"
          v-model:username="davUsername"
          v-model:secret="davSecret"
        />

        <button
          type="button"
          class="button"
          :disabled="verifying || !davCanCheck"
          @click="onCheckDavShelf"
        >
          {{ verifying ? t('shelfSource.checking') : t('shelfSource.check') }}
        </button>

        <p v-if="davBookCount !== null" class="mobile-shelf-form-hint" role="status">
          {{ t('shelfSource.shelfFound', { count: String(davBookCount) }) }}
        </p>
        <p class="mobile-shelf-form-hint">{{ t('shelfSource.readOnlyHint') }}</p>
      </template>

      <template v-else>
        <label class="mobile-shelf-form-field">
          <span class="mobile-shelf-form-label">{{ t('mobileShelfForm.pcloud.clientIdLabel') }}</span>
          <input
            v-model="pcloudClientId"
            class="input"
            type="text"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileShelfForm.pcloud.clientIdPlaceholder')"
          />
          <span class="mobile-shelf-form-hint">{{ t('mobileShelfForm.pcloud.clientIdHint') }}</span>
        </label>

        <button
          type="button"
          class="button"
          :disabled="authorizing || pcloudClientId.trim().length === 0"
          @click="onAuthorize"
        >
          {{ authorizing ? t('mobileShelfForm.pcloud.authorizing') : t('mobileShelfForm.pcloud.authorize') }}
        </button>

        <button v-if="authorizing" type="button" class="button" @click="onCancelAuthorize">
          {{ t('mobileShelfForm.pcloud.cancel') }}
        </button>

        <p v-if="isAuthorized" class="mobile-shelf-form-hint" role="status">
          {{
            pcloudAccountIdentity
              ? t('mobileShelfForm.pcloud.authorizedAccount', {
                  account: pcloudAccountIdentity,
                  host: pcloudHost
                })
              : t('mobileShelfForm.pcloud.authorized', { host: pcloudHost })
          }}
        </p>

        <label class="mobile-shelf-form-field">
          <span class="mobile-shelf-form-label">{{ t('mobileShelfForm.pcloud.shelfRootLabel') }}</span>
          <input
            v-model="pcloudShelfRoot"
            class="input"
            type="text"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            :placeholder="t('mobileShelfForm.pcloud.shelfRootPlaceholder')"
            @input="pcloudBookCount = null"
          />
          <span class="mobile-shelf-form-hint">{{ t('mobileShelfForm.pcloud.shelfRootHint') }}</span>
        </label>

        <button type="button" class="button" :disabled="!isAuthorized" @click="pickerOpen = true">
          {{ t('mobileShelfForm.pcloud.picker.browse') }}
        </button>

        <button
          type="button"
          class="button"
          :disabled="verifying || !isAuthorized || pcloudShelfRoot.trim().length === 0"
          @click="onVerifyPCloudShelf"
        >
          {{ verifying ? t('mobileShelfForm.pcloud.verifying') : t('mobileShelfForm.pcloud.verify') }}
        </button>

        <p v-if="pcloudBookCount !== null" class="mobile-shelf-form-hint" role="status">
          {{ t('mobileShelfForm.pcloud.shelfFound', { count: String(pcloudBookCount) }) }}
        </p>

        <PCloudFolderPicker
          :open="pickerOpen"
          :host="pcloudHost"
          :access-token="pcloudAccessToken"
          :initial-path="pcloudShelfRoot"
          @select="onPickShelfRoot"
          @close="pickerOpen = false"
        />
      </template>

      <p v-if="entry && type !== 'local'" class="mobile-shelf-form-hint">{{ t('mobileShelfForm.retargetHint') }}</p>

      <p v-if="message" class="mobile-shelf-form-error" role="alert">{{ message }}</p>

      <button type="submit" class="button primary" :disabled="saving || !canSave">
        {{ saving ? t('mobileShelfForm.saving') : t('mobileShelfForm.save') }}
      </button>

      <button v-if="cancellable" type="button" class="button" @click="emit('cancel')">
        {{ t('mobileShelfForm.cancel') }}
      </button>
      </fieldset>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { Browser } from '@capacitor/browser';
import { Capacitor } from '@capacitor/core';
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
import { surfaceError } from '@/composables/useErrorIncident';
import { useI18n } from '@/i18n';
import { listServerShelves, type ShelfInfo } from '@/api/shelves';
import { buildAuthorizeUrl, generateRequestId, pollForToken } from '@/api/pcloud/auth';
import { PCloudClient } from '@/api/pcloud/client';
import { LocalShelf } from '@/providers/localShelf';
import {
  getShelfEntryToken,
  newShelfEntry,
  upsertShelfEntry,
  type ShelfEntry,
  type ShelfSourceType
} from '@/providers/mobileConfig';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';
import { checkPCloudShelf } from '@/features/mobile/utils/pcloudSetup';
import {
  shelfSourceCheckMessageKey,
  shelfSourceNeedsSecret,
  type ShelfSourceKind
} from '@/utils/shelfSourceCheck';
import MobileQRImport from '@/features/mobile/components/MobileQRImport.vue';
import RemoteSourceFields from '@/components/shelfSource/RemoteSourceFields.vue';
import ShelfSourceModes from '@/components/shelfSource/ShelfSourceModes.vue';
import PCloudFolderPicker from '@/features/mobile/components/PCloudFolderPicker.vue';

const props = defineProps<{
  entry?: ShelfEntry | null;
  cancellable?: boolean;
}>();

const emit = defineEmits<{ (event: 'cancel'): void }>();

const { t } = useI18n();

const entry = computed(() => props.entry ?? null);

type FormMode = ShelfSourceType | 'qr';

const native = Capacitor.isNativePlatform();
const typeOptions = computed(() => [
  ...(native
    ? [{ value: 'local' as const, label: t('mobileShelfForm.modeLocal'), help: t('mobileShelfForm.modeLocalHelp') }]
    : []),
  { value: 'server' as const, label: t('mobileShelfForm.modeServer'), help: t('mobileShelfForm.modeServerHelp') },
  { value: 'pcloud' as const, label: t('mobileShelfForm.modePCloud'), help: t('mobileShelfForm.modePCloudHelp') },
  ...(native
    ? [
        { value: 'dav' as const, label: t('shelfSource.modeRemote'), help: t('shelfSource.modeRemoteHelp') },
        { value: 'qr' as const, label: t('mobileQR.scan'), help: t('shelfSource.modeQRHelp') }
      ]
    : [])
]);

const type = ref<FormMode>(typeOptions.value[0].value);
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
const pcloudUserId = ref('');
const pcloudAccountIdentity = ref('');
const pcloudBookCount = ref<number | null>(null);
const pickerOpen = ref(false);

const davSource = ref<ShelfSourceKind>('plainshelf');
const davUrl = ref('');
const davUsername = ref('');
const davSecret = ref('');
const davBookCount = ref<number | null>(null);
let davDraftRevision = 0;
const davAccount = () => (davSource.value === 'webdav' ? davUsername.value.trim() : '');
const davCanCheck = computed(
  () =>
    davUrl.value.trim().length > 0 &&
    (!shelfSourceNeedsSecret(davSource.value, davAccount()) || davSecret.value.trim().length > 0)
);
watch([davSource, davUrl, davUsername, davSecret], () => {
  davDraftRevision++;
  davBookCount.value = null;
});

const qrBusy = ref(false);
const saving = ref(false);
const authorizing = ref(false);
const verifying = ref(false);
const localError = ref('');

let authAbort: AbortController | null = null;

const heading = computed(() =>
  entry.value ? t('mobileShelfForm.editTitle') : t('mobileShelfForm.title')
);
const isAuthorized = computed(() => Boolean(pcloudAccessToken.value && pcloudHost.value));
const message = computed(() => localError.value);

const canSave = computed(() => {
  if (type.value === 'local') {
    return name.value.trim().length > 0;
  }
  if (type.value === 'dav') {
    return davCanCheck.value;
  }
  return type.value === 'server'
    ? serverUrl.value.trim().length > 0 && shelfId.value.length > 0
    : isAuthorized.value && pcloudShelfRoot.value.trim().length > 0 && pcloudBookCount.value !== null;
});

onMounted(async () => {
  const existing = entry.value;
  if (!existing) {
    return;
  }

  type.value = existing.type;
  name.value = existing.name;
  if (existing.type === 'local') {
    return;
  }
  const secret = await getShelfEntryToken(existing.id);

  if (existing.type === 'dav') {
    davSource.value = existing.davSource;
    davUrl.value = existing.davUrl;
    davUsername.value = existing.davUsername ?? '';
    davSecret.value = secret;
    return;
  }

  if (existing.type === 'pcloud') {
    pcloudClientId.value = existing.pcloudClientId;
    pcloudHost.value = existing.pcloudHost;
    pcloudShelfRoot.value = existing.pcloudShelfRoot;
    pcloudAccessToken.value = secret;
    pcloudUserId.value = existing.pcloudUserId ?? '';
    if (secret && existing.pcloudHost) {
      try {
        const account = await loadPCloudAccountIdentity(existing.pcloudHost, secret);
        pcloudAccountIdentity.value = account.label;
      } catch (err) {
        localError.value = surfaceError(err);
      }
    }
    return;
  }

  serverUrl.value = existing.serverUrl;
  shelfId.value = existing.shelfId;
  token.value = secret;
  if (existing.serverUrl) {
    await loadShelves();
  }
});

function onTypeSelect(next: FormMode): void {
  type.value = next;
  localError.value = '';
}

async function loadShelves(): Promise<void> {
  shelvesLoading.value = true;
  try {
    const loaded = await listServerShelves({ apiBase: serverUrl.value, token: token.value });
    shelves.value = loaded;
    if (!loaded.some((shelf) => shelf.id === shelfId.value)) {
      shelfId.value = '';
    }
  } catch (err) {
    shelves.value = [];
    shelfId.value = '';
    localError.value = surfaceError(err);
  } finally {
    shelvesLoading.value = false;
  }
}

async function onLoadShelves(): Promise<void> {
  localError.value = '';
  if (serverUrl.value.trim().length === 0) {
    localError.value = t('mobileShelfForm.serverUrlRequired');
    return;
  }
  await loadShelves();
}

function onShelfSelect(value: AcceptableValue): void {
  if (typeof value === 'string') {
    shelfId.value = value;
  }
}

async function loadPCloudAccountIdentity(
  host: string,
  accessToken: string
): Promise<{ label: string; userId: string }> {
  const info = await new PCloudClient({ host, accessToken }).getUserInfo();
  const userId = info.userid === undefined ? '' : String(info.userid);
  return { label: info.email?.trim() || userId, userId };
}

async function onAuthorize(): Promise<void> {
  localError.value = '';
  const clientId = pcloudClientId.value.trim();
  if (!clientId) {
    localError.value = t('mobileShelfForm.pcloud.clientIdRequired');
    return;
  }

  authorizing.value = true;
  authAbort = new AbortController();
  try {
    const requestId = generateRequestId();
    await Browser.open({ url: buildAuthorizeUrl(clientId, requestId) });
    const session = await pollForToken({ clientId, requestId, signal: authAbort.signal });
    const account = await loadPCloudAccountIdentity(session.host, session.accessToken);

    pcloudAccessToken.value = session.accessToken;
    pcloudHost.value = session.host;
    pcloudAccountIdentity.value = account.label;
    pcloudUserId.value = account.userId;
    pcloudBookCount.value = null;
    await closeBrowser();
  } catch (err) {
    localError.value = surfaceError(err);
  } finally {
    authorizing.value = false;
    authAbort = null;
  }
}

function onCancelAuthorize(): void {
  authAbort?.abort();
}

async function closeBrowser(): Promise<void> {
  try {
    await Browser.close();
  } catch {
  }
}

async function onVerifyPCloudShelf(): Promise<void> {
  localError.value = '';
  pcloudBookCount.value = null;
  const shelfRoot = pcloudShelfRoot.value.trim();
  if (!shelfRoot) {
    localError.value = t('mobileShelfForm.pcloud.shelfRootRequired');
    return;
  }

  verifying.value = true;
  try {
    const books = await checkPCloudShelf(pcloudHost.value, pcloudAccessToken.value, shelfRoot);
    if (books === null) {
      localError.value = t('mobileShelfForm.pcloud.notAShelf');
      return;
    }
    pcloudBookCount.value = books;
  } catch (err) {
    localError.value = surfaceError(err);
  } finally {
    verifying.value = false;
  }
}

async function onPickShelfRoot(path: string): Promise<void> {
  pickerOpen.value = false;
  pcloudShelfRoot.value = path;
  pcloudBookCount.value = null;
  await onVerifyPCloudShelf();
}

async function checkDavShelf(): Promise<boolean> {
  localError.value = '';
  davBookCount.value = null;
  verifying.value = true;
  const revision = ++davDraftRevision;
  try {
    const check = await LocalShelf.checkSource({
      source: davSource.value,
      url: davUrl.value.trim(),
      username: davAccount(),
      secret: davSecret.value.trim()
    });
    if (revision !== davDraftRevision) {
      return false;
    }
    const messageKey = shelfSourceCheckMessageKey(check);
    if (messageKey) {
      localError.value = t(messageKey);
      return false;
    }
    davBookCount.value = check.books;
    return true;
  } catch (err) {
    localError.value = surfaceError(err);
    return false;
  } finally {
    verifying.value = false;
  }
}

async function onCheckDavShelf(): Promise<void> {
  await checkDavShelf();
}

function draftEntry(kind: 'server' | 'pcloud' | 'dav'): { entry: ShelfEntry; secret: string } {
  const base = entry.value ?? newShelfEntry(kind);
  if (kind === 'dav') {
    const username = davAccount();
    return {
      entry: {
        id: base.id,
        type: 'dav',
        name: name.value.trim(),
        davSource: davSource.value,
        davUrl: davUrl.value.trim(),
        ...(username ? { davUsername: username } : {})
      },
      secret: davSecret.value.trim()
    };
  }
  if (kind === 'pcloud') {
    return {
      entry: {
        id: base.id,
        type: 'pcloud',
        name: name.value.trim(),
        pcloudClientId: pcloudClientId.value.trim(),
        pcloudHost: pcloudHost.value,
        pcloudShelfRoot: pcloudShelfRoot.value.trim(),
        ...(pcloudUserId.value ? { pcloudUserId: pcloudUserId.value } : {})
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

async function saveLocalShelf(): Promise<void> {
  const trimmed = name.value.trim();
  const existing = entry.value;
  let localShelfId: string;
  if (existing?.type === 'local') {
    localShelfId = existing.localShelfId;
    await LocalShelf.renameShelf({ id: localShelfId, name: trimmed });
  } else {
    ({ id: localShelfId } = await LocalShelf.createShelf({ name: trimmed }));
  }
  const base = existing ?? newShelfEntry('local');
  await upsertShelfEntry({ id: base.id, type: 'local', name: trimmed, localShelfId });
}

async function onSave(): Promise<void> {
  localError.value = '';

  if (type.value === 'local') {
    if (!canSave.value) {
      localError.value = t('mobileShelfForm.localNameRequired');
      return;
    }
    saving.value = true;
    try {
      await saveLocalShelf();
      reloadIntoApp();
    } catch (err) {
      localError.value = surfaceError(err);
      saving.value = false;
    }
    return;
  }

  const kind = type.value;
  if (kind === 'qr') {
    return;
  }
  if (kind === 'server' && shelfId.value.length === 0) {
    localError.value = t('mobileShelfForm.shelfRequired');
    return;
  }
  if (kind === 'pcloud' && !canSave.value) {
    localError.value = t('mobileShelfForm.pcloud.verifyRequired');
    return;
  }

  saving.value = true;
  try {
    if (kind === 'dav' && davBookCount.value === null && !(await checkDavShelf())) {
      saving.value = false;
      return;
    }
    const draft = draftEntry(kind);
    await upsertShelfEntry(draft.entry, draft.secret);
    reloadIntoApp();
  } catch (err) {
    localError.value = surfaceError(err);
    saving.value = false;
  }
}
</script>

<style scoped>
.mobile-shelf-form-card {
  width: min(480px, 100%);
  padding: 24px;
  display: grid;
  gap: 12px;
}

.mobile-shelf-form-title {
  margin: 0;
  font-size: 1.4rem;
}

.mobile-shelf-form-description {
  margin: 0;
  color: var(--text-muted, #666);
}

.mobile-shelf-form-form {
  display: grid;
  gap: 16px;
  margin-top: 8px;
}

.mobile-shelf-form-fields {
  display: grid;
  gap: 16px;
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}

.mobile-shelf-form-field {
  display: grid;
  gap: 6px;
}

.mobile-shelf-form-label {
  font-weight: 600;
  font-size: 0.9rem;
}

.mobile-shelf-form-hint {
  font-size: 0.8rem;
  color: var(--text-muted, #666);
}

.mobile-shelf-form-shelf-select {
  width: 100%;
  justify-content: space-between;
}

.mobile-shelf-form-error {
  margin: 0;
  color: var(--danger, #b3261e);
  font-size: 0.9rem;
}
</style>
