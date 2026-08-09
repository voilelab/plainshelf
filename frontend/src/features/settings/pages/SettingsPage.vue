<template>
  <section class="settings-page">
    <DeleteModal
      :open="pendingRemoveShelf !== null"
      :title="t('settings.shelves.removeShelfTitle')"
      :item-name="pendingRemoveShelf?.name ?? ''"
      :description="pendingRemoveShelf ? t('settings.shelves.removeConfirmDescription') : ''"
      :confirm-text="t('settings.shelves.removeConfirmYes')"
      :cancel-text="t('common.cancel')"
      :busy-text="t('settings.shelves.removing')"
      :busy="pendingRemoveShelf ? removingShelfIDs.has(pendingRemoveShelf.id) : false"
      :error="shelfRemoveModalError"
      @cancel="cancelRemoveShelf"
      @confirm="confirmRemoveShelf"
    />

    <ConfirmModal
      :open="showModifyShelfModal"
      :title="t('settings.shelves.modifyShelfTitle')"
      :confirm-text="t('settings.shelves.modifyShelfSubmit')"
      :cancel-text="t('common.cancel')"
      :busy-text="t('settings.shelves.modifyShelfSaving')"
      :busy="modifyingShelf"
      :confirm-disabled="!canSubmitModifyShelf"
      :close-label="t('settings.shelves.modifyShelfCloseLabel')"
      @cancel="closeModifyShelfModal"
      @confirm="onSubmitModifyShelf"
    >
      <form class="shelf-add-form" @submit.prevent="onSubmitModifyShelf">
        <div class="shelf-modify-readonly-row">
          <span class="shelf-modify-label">{{ t('settings.shelves.modifyShelfIDLabel') }}</span>
          <span class="shelf-modify-value shelf-id-cell">{{ pendingModifyShelf?.id }}</span>
        </div>
        <div class="shelf-modify-readonly-row">
          <span class="shelf-modify-label">{{ t('settings.shelves.modifyShelfPathLabel') }}</span>
          <span class="shelf-modify-value">{{ modifyShelfPath }}</span>
        </div>
        <input
          v-model="modifyShelfName"
          class="shelf-add-input"
          type="text"
          :placeholder="t('settings.shelves.modifyShelfNamePlaceholder')"
          :disabled="modifyingShelf"
          autofocus
        />
        <input
          v-model="modifyShelfScanInterval"
          class="shelf-add-input"
          type="text"
          :placeholder="t('settings.shelves.modifyShelfScanIntervalPlaceholder')"
          :disabled="modifyingShelf"
        />
        <p class="shelf-add-help">{{ t('settings.shelves.modifyShelfScanIntervalHelp') }}</p>
        <p v-if="modifyShelfError" class="settings-message settings-message-error" role="alert">
          {{ modifyShelfError }}
        </p>
      </form>
    </ConfirmModal>

    <ConfirmModal
      :open="showAddShelfModal"
      :title="t('settings.shelves.addShelfTitle')"
      :confirm-text="t('settings.shelves.addShelfSubmit')"
      :cancel-text="t('common.cancel')"
      :busy-text="t('settings.shelves.addShelfAdding')"
      :busy="addingShelf"
      :confirm-disabled="!canSubmitAddShelf"
      :close-label="t('settings.shelves.addShelfCloseLabel')"
      @cancel="closeAddShelfModal"
      @confirm="onSubmitAddShelf"
    >
      <form class="shelf-add-form" @submit.prevent="onSubmitAddShelf">
        <input
          v-model="newShelfName"
          class="shelf-add-input"
          type="text"
          :placeholder="t('settings.shelves.addShelfNamePlaceholder')"
          :disabled="addingShelf"
          autofocus
        />
        <div class="shelf-add-dir-row">
          <input
            v-model="newShelfDirectory"
            class="shelf-add-input shelf-add-dir-input"
            type="text"
            :placeholder="t('settings.shelves.addShelfDirectoryPlaceholder')"
            :disabled="addingShelf"
          />
          <button
            type="button"
            class="shelf-browse-btn"
            :disabled="addingShelf"
            @click="onBrowseShelfDirectory"
          >
            {{ t('settings.shelves.addShelfBrowse') }}
          </button>
        </div>
        <input
          v-model="newShelfScanInterval"
          class="shelf-add-input"
          type="text"
          :placeholder="t('settings.shelves.addShelfScanIntervalPlaceholder')"
          :disabled="addingShelf"
        />
        <p class="shelf-add-help">{{ t('settings.shelves.addShelfScanIntervalHelp') }}</p>
        <p v-if="addShelfError" class="settings-message settings-message-error" role="alert">
          {{ addShelfError }}
        </p>
      </form>
    </ConfirmModal>

    <FontLicenseModal
      :open="selectedFontLicense !== null"
      :title="t('settings.about.licenseTitle', { font: selectedFontLicense?.name ?? '' })"
      :text="fontLicenseText"
      :loading="fontLicenseLoading"
      :error="fontLicenseError"
      :loading-label="t('settings.about.licenseLoading')"
      :close-label="t('settings.about.licenseClose')"
      @close="closeFontLicense"
    />

    <header class="settings-header">
      <div>
        <h2>{{ t('settings.title') }}</h2>
        <p>{{ t('settings.description') }}</p>
      </div>
      <button class="button" type="button" :disabled="loading || saving" @click="loadSettings">
        {{ t('common.retry') }}
      </button>
    </header>

    <p v-if="error" class="settings-message settings-message-error" role="alert">
      {{ error }}
    </p>

    <TabsRoot :default-value="defaultSettingsTab" class="settings-tabs">
      <TabsList class="settings-tabs-list" :aria-label="t('settings.title')">
        <TabsTrigger v-if="serverSettingsEditable" value="cover" class="settings-tab-trigger">{{
          t('settings.cover.title')
        }}</TabsTrigger>
        <!-- Reading history is device-local, so its retention limit is
             editable on every client, including the read-only mobile shell. -->
        <TabsTrigger value="read-history" class="settings-tab-trigger">{{
          t('settings.readHistory.title')
        }}</TabsTrigger>
        <TabsTrigger v-if="serverSettingsEditable" value="reader" class="settings-tab-trigger">{{
          t('settings.reader.title')
        }}</TabsTrigger>
        <TabsTrigger v-if="serverSettingsEditable" value="import" class="settings-tab-trigger">{{
          t('settings.import.title')
        }}</TabsTrigger>
        <TabsTrigger value="about" class="settings-tab-trigger">{{ t('settings.about.title') }}</TabsTrigger>
        <TabsTrigger value="shelves" class="settings-tab-trigger">{{ t('settings.shelves.title') }}</TabsTrigger>
      </TabsList>

      <TabsContent v-if="serverSettingsEditable" value="cover" class="settings-tab-content">
        <section class="panel settings-group">
          <h3>{{ t('settings.cover.title') }}</h3>
          <label class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.coverToJpg.label') }}</div>
              <p class="setting-description">{{ t('settings.coverToJpg.description') }}</p>
            </div>
            <input
              class="setting-checkbox"
              type="checkbox"
              :checked="coverToJpg"
              :disabled="loading || saving"
              @change="onCoverToJpgChange"
            />
          </label>
        </section>
      </TabsContent>

      <TabsContent value="read-history" class="settings-tab-content">
        <section class="panel settings-group">
          <h3>{{ t('settings.readHistory.title') }}</h3>
          <label class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.readHistoryLimit.label') }}</div>
              <p class="setting-description">{{ t('settings.readHistoryLimit.description') }}</p>
            </div>
            <input
              class="setting-number"
              type="number"
              inputmode="numeric"
              min="0"
              step="1"
              :value="readHistoryLimit"
              :disabled="loading || saving"
              @change="onReadHistoryLimitChange"
            />
          </label>
        </section>
      </TabsContent>

      <TabsContent v-if="serverSettingsEditable" value="reader" class="settings-tab-content">
        <section class="panel settings-group">
          <h3>{{ t('settings.defaultSplitConfig.label') }}</h3>
          <p class="setting-description">{{ t('settings.defaultSplitConfig.description') }}</p>

          <p v-if="splitConfigError" class="settings-message settings-message-error" role="alert">
            {{ splitConfigError }}
          </p>

          <label class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.defaultSplitConfig.typeLabel') }}</div>
            </div>
            <select
              class="setting-select"
              :value="defaultSplitType"
              :disabled="loading || saving"
              @change="onDefaultSplitTypeChange"
            >
              <option value="none">{{ t('settings.defaultSplitConfig.typeNone') }}</option>
              <option value="line_count">{{ t('settings.defaultSplitConfig.typeLineCount') }}</option>
              <option value="regex">{{ t('settings.defaultSplitConfig.typeRegex') }}</option>
            </select>
          </label>

          <label v-if="defaultSplitType === 'line_count'" class="setting-item setting-item-stacked">
            <div>
              <div class="setting-label">{{ t('settings.defaultSplitConfig.lineCountLabel') }}</div>
            </div>
            <input
              v-model="defaultSplitLineCount"
              class="setting-number"
              type="number"
              min="1"
              step="1"
              :placeholder="t('settings.defaultSplitConfig.lineCountPlaceholder')"
              :disabled="loading || saving"
            />
          </label>

          <div v-if="defaultSplitType === 'regex'" class="setting-item-stacked">
            <label>
              <div class="setting-label">{{ t('settings.defaultSplitConfig.regexLabel') }}</div>
              <p class="setting-description">{{ t('settings.defaultSplitConfig.regexHelp') }}</p>
            </label>
            <textarea
              v-model="defaultSplitRegex"
              class="setting-textarea"
              rows="3"
              :placeholder="t('settings.defaultSplitConfig.regexPlaceholder')"
              :disabled="loading || saving"
            />
          </div>

          <div class="split-config-actions">
            <button
              class="button primary"
              type="button"
              :disabled="loading || saving"
              @click="onSaveDefaultSplitConfig"
            >
              {{ saving ? t('settings.defaultSplitConfig.saving') : t('settings.defaultSplitConfig.save') }}
            </button>
          </div>
        </section>
      </TabsContent>

      <TabsContent v-if="serverSettingsEditable" value="import" class="settings-tab-content">
        <section class="panel settings-group">
          <h3>{{ t('settings.epubImport.title') }}</h3>
          <p class="setting-description">{{ t('settings.epubImport.description') }}</p>

          <label class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.epubImport.presetLabel') }}</div>
              <p class="setting-description">{{ t('settings.epubImport.presetHelp') }}</p>
            </div>
            <select
              class="setting-select"
              :value="epubPreset"
              :disabled="loading || saving"
              @change="onEpubPresetChange"
            >
              <option value="markdown">{{ t('settings.epubImport.presetMarkdown') }}</option>
              <option value="plain">{{ t('settings.epubImport.presetPlain') }}</option>
            </select>
          </label>

          <label class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.epubImport.includeDescriptionLabel') }}</div>
              <p class="setting-description">{{ t('settings.epubImport.includeDescriptionHelp') }}</p>
            </div>
            <input
              v-model="epubIncludeDescription"
              class="setting-checkbox"
              type="checkbox"
              :disabled="loading || saving"
            />
          </label>

          <p v-if="epubImportError" class="error">{{ epubImportError }}</p>

          <div class="split-config-actions">
            <button
              class="button primary"
              type="button"
              :disabled="loading || saving"
              @click="onSaveEpubImportStrategy"
            >
              {{ saving ? t('settings.epubImport.saving') : t('settings.epubImport.save') }}
            </button>
          </div>
        </section>
      </TabsContent>

      <TabsContent value="about" class="settings-tab-content">
        <section class="panel settings-group">
          <h3>{{ t('settings.about.title') }}</h3>
          <p class="setting-description">{{ t('settings.about.description') }}</p>
          <div class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.about.version') }}</div>
            </div>
            <span class="setting-value">{{ version || '—' }}</span>
          </div>
          <div class="setting-item">
            <div>
              <div class="setting-label">{{ t('settings.about.repository') }}</div>
            </div>
            <a class="setting-link" :href="githubRepoUrl" target="_blank" rel="noreferrer" @click="onRepositoryLinkClick">
              {{ githubRepoUrl }}
            </a>
          </div>
          <div class="font-license-section">
            <div class="setting-label">{{ t('settings.about.thirdPartyFonts') }}</div>
            <article v-for="font in bundledFonts" :key="font.name" class="font-license-item">
              <strong>{{ font.name }}</strong>
              <span class="setting-description">{{ t('settings.about.fontAttribution') }}</span>
              <span class="font-license-links">
                <a
                  class="setting-link"
                  :href="font.source"
                  target="_blank"
                  rel="noreferrer"
                  @click="onBundledFontSourceClick($event, font.source)"
                >
                  {{ t('settings.about.source') }}
                </a>
                <a class="setting-link" :href="font.license" @click.prevent="openFontLicense(font)">
                  {{ t('settings.about.license') }}
                </a>
              </span>
            </article>
          </div>
        </section>
      </TabsContent>

      <TabsContent value="shelves" class="settings-tab-content">
        <section v-if="isMobileEnv" class="panel settings-group">
          <h3>{{ t('settings.mobileConnect.title') }}</h3>
          <p class="setting-description">{{ t('settings.mobileConnect.description') }}</p>
          <RouterLink to="/connect" class="button mobile-connect-link">
            {{ t('settings.mobileConnect.open') }}
          </RouterLink>
        </section>

        <section v-if="isMobileEnv" class="panel settings-group">
          <h3>{{ t('settings.downloads.title') }}</h3>
          <p class="setting-description">{{ t('settings.downloads.description') }}</p>
          <RouterLink to="/downloads" class="button mobile-connect-link">
            {{ t('settings.downloads.open') }}
          </RouterLink>
        </section>

        <section class="panel settings-group">
          <h3>{{ t('settings.shelves.title') }}</h3>

          <p v-if="shelvesError || shelfOpError" class="settings-message settings-message-error" role="alert">
            {{ shelfOpError || shelvesError }}
          </p>

          <p v-if="!isDesktopEnv" class="setting-description">
            {{ t('settings.shelves.serverManaged') }}
          </p>

          <div v-if="shelvesLoading" class="setting-description">{{ t('layout.shelf.loading') }}</div>
          <template v-else>
            <p v-if="shelves.length === 0" class="setting-description">
              {{ t('settings.shelves.empty') }}
            </p>
            <table v-else class="shelves-table">
              <thead>
                <tr>
                  <th>{{ t('settings.shelves.name') }}</th>
                  <th>ID</th>
                  <th v-if="isDesktopEnv"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="shelf in shelves" :key="shelf.id">
                  <td>{{ shelf.name }}</td>
                  <td class="shelf-id-cell">{{ shelf.id }}</td>
                  <td v-if="isDesktopEnv" class="shelf-action-cell">
                    <button
                      type="button"
                      class="shelf-modify-btn"
                      :disabled="removingShelfIDs.has(shelf.id)"
                      @click="requestModifyShelf(shelf)"
                    >
                      {{ t('settings.shelves.modify') }}
                    </button>
                    <button
                      type="button"
                      class="shelf-remove-btn"
                      :disabled="removingShelfIDs.has(shelf.id)"
                      @click="requestRemoveShelf(shelf)"
                    >
                      {{ t('settings.shelves.remove') }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </template>

          <div v-if="isDesktopEnv" class="shelf-add-row">
            <button
              type="button"
              class="shelf-add-toggle"
              :disabled="addingShelf"
              @click="openAddShelfModal"
            >
              {{ t('settings.shelves.addShelf') }}
            </button>
          </div>
        </section>
      </TabsContent>
    </TabsRoot>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui';
import ConfirmModal from '@/components/ConfirmModal.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import FontLicenseModal from '@/features/settings/components/FontLicenseModal.vue';
import {
  getCoverToJpgSetting,
  getDefaultSplitConfigSetting,
  getEpubImportStrategySetting,
  setCoverToJpgSetting,
  setDefaultSplitConfigSetting,
  setEpubImportStrategySetting
} from '@/api/settings';
// Reading history and its retention limit are per-device state, not server settings.
import { getReadHistoryLimit, setReadHistoryLimit } from '@/storage/readHistory';
import {
  DEFAULT_EPUB_IMPORT_STRATEGY,
  type EpubImportPreset,
  type EpubImportStrategy,
  type SplitConfig,
  type SplitType
} from '@/types/book';
import { normalizeEpubImportPreset } from '@/utils/epubStrategy';
import { getServerVersion } from '@/api/version';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useI18n } from '@/i18n';
import { isMobileRuntime, isWailsRuntime } from '@/providers';
import { useShelfManagement } from '@/features/settings/composables/useShelfManagement';
import { openExternalURL } from '@/features/settings/utils/externalLinks';
import {
  buildDefaultSplitConfig,
  parseReadHistoryLimit
} from '@/features/settings/utils/settingsDraft';

const { t } = useI18n();
const loading = ref(false);
const saving = ref(false);
const error = ref('');
const coverToJpg = ref(false);
const readHistoryLimit = ref(0);
const defaultSplitType = ref<SplitType>('none');
const defaultSplitLineCount = ref('100');
const defaultSplitRegex = ref('');
const splitConfigError = ref('');
const epubPreset = ref<EpubImportPreset>(DEFAULT_EPUB_IMPORT_STRATEGY.preset);
const epubIncludeDescription = ref(DEFAULT_EPUB_IMPORT_STRATEGY.include_description);
const epubImportError = ref('');
const version = ref('');
const githubRepoUrl = 'https://github.com/voilelab/plainshelf';
interface BundledFont {
  name: string;
  source: string;
  license: string;
}

const bundledFonts: BundledFont[] = [
  {
    name: 'Noto Serif TC',
    source: 'https://fontsource.org/fonts/noto-serif-tc',
    license: '/licenses/noto-serif-tc-OFL-1.1.txt'
  },
  {
    name: 'Noto Sans TC',
    source: 'https://fontsource.org/fonts/noto-sans-tc',
    license: '/licenses/noto-sans-tc-OFL-1.1.txt'
  }
];
const selectedFontLicense = ref<BundledFont | null>(null);
const fontLicenseText = ref('');
const fontLicenseLoading = ref(false);
const fontLicenseError = ref('');
const fontLicenseCache = new Map<string, string>();
let fontLicenseRequest = 0;

const isDesktopEnv = computed(() => isWailsRuntime());
const isMobileEnv = computed(() => isMobileRuntime());
// Shelves is the useful landing tab when the server settings tabs are gone:
// it holds the connection and downloads panels.
const { serverSettingsEditable } = useWriteAccess();
const defaultSettingsTab = computed(() => (serverSettingsEditable.value ? 'cover' : 'shelves'));
const {
  shelves,
  shelvesLoading,
  shelvesError,
  fetchShelves,
  shelfOpError,
  removingShelfIDs,
  pendingRemoveShelf,
  shelfRemoveModalError,
  requestRemoveShelf,
  cancelRemoveShelf,
  confirmRemoveShelf,
  showAddShelfModal,
  newShelfName,
  newShelfDirectory,
  newShelfScanInterval,
  addingShelf,
  addShelfError,
  canSubmitAddShelf,
  openAddShelfModal,
  closeAddShelfModal,
  onBrowseShelfDirectory,
  onSubmitAddShelf,
  pendingModifyShelf,
  showModifyShelfModal,
  modifyShelfName,
  modifyShelfScanInterval,
  modifyShelfPath,
  modifyingShelf,
  modifyShelfError,
  canSubmitModifyShelf,
  requestModifyShelf,
  closeModifyShelfModal,
  onSubmitModifyShelf
} = useShelfManagement();

useDocumentTitle(() => [t('settings.title'), t('app.name')]);

function onRepositoryLinkClick(event: MouseEvent): void {
  event.preventDefault();
  void openExternalURL(githubRepoUrl);
}

function onBundledFontSourceClick(event: MouseEvent, url: string): void {
  event.preventDefault();
  void openExternalURL(url);
}

async function openFontLicense(font: BundledFont): Promise<void> {
  selectedFontLicense.value = font;
  fontLicenseError.value = '';

  const cachedText = fontLicenseCache.get(font.license);
  if (cachedText !== undefined) {
    fontLicenseText.value = cachedText;
    fontLicenseLoading.value = false;
    return;
  }

  const request = ++fontLicenseRequest;
  fontLicenseText.value = '';
  fontLicenseLoading.value = true;

  try {
    const response = await fetch(font.license);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const text = await response.text();
    fontLicenseCache.set(font.license, text);
    if (request === fontLicenseRequest) {
      fontLicenseText.value = text;
    }
  } catch {
    if (request === fontLicenseRequest) {
      fontLicenseError.value = t('settings.about.licenseLoadFailed');
    }
  } finally {
    if (request === fontLicenseRequest) {
      fontLicenseLoading.value = false;
    }
  }
}

function closeFontLicense(): void {
  fontLicenseRequest += 1;
  selectedFontLicense.value = null;
  fontLicenseLoading.value = false;
}

async function loadSettings(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    // The retention limit is device-local, so it loads even where the
    // server-only settings are not shown (the mobile shell).
    readHistoryLimit.value = await getReadHistoryLimit();
    if (!serverSettingsEditable.value) {
      return;
    }

    const [nextCoverToJpg, nextDefaultSplitConfig, nextEpubStrategy] = await Promise.all([
      getCoverToJpgSetting(),
      getDefaultSplitConfigSetting(),
      getEpubImportStrategySetting()
    ]);
    coverToJpg.value = nextCoverToJpg;
    hydrateSplitConfigDraft(nextDefaultSplitConfig);
    hydrateEpubImportDraft(nextEpubStrategy);
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('settings.loadFailed');
  } finally {
    loading.value = false;
  }
}

function hydrateSplitConfigDraft(config: SplitConfig): void {
  defaultSplitType.value = config.type;
  defaultSplitLineCount.value = String(config.line_count ?? 100);
  defaultSplitRegex.value = config.regex ?? '';
}

function hydrateEpubImportDraft(strategy: EpubImportStrategy): void {
  epubPreset.value = strategy.preset;
  epubIncludeDescription.value = strategy.include_description;
}

function onEpubPresetChange(event: Event): void {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }
  epubPreset.value = normalizeEpubImportPreset(target.value);
  epubImportError.value = '';
}

async function onSaveEpubImportStrategy(): Promise<void> {
  epubImportError.value = '';
  saving.value = true;

  try {
    await setEpubImportStrategySetting({
      preset: epubPreset.value,
      include_description: epubIncludeDescription.value
    });
  } catch (err) {
    epubImportError.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

function onDefaultSplitTypeChange(event: Event): void {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }
  defaultSplitType.value = target.value as SplitType;
  splitConfigError.value = '';
}

async function onSaveDefaultSplitConfig(): Promise<void> {
  splitConfigError.value = '';

  const result = buildDefaultSplitConfig({
    type: defaultSplitType.value,
    lineCount: defaultSplitLineCount.value,
    regex: defaultSplitRegex.value
  });
  if (!result.ok) {
    splitConfigError.value = t(`settings.defaultSplitConfig.${result.error}`);
    return;
  }

  saving.value = true;
  try {
    await setDefaultSplitConfigSetting(result.config);
  } catch (err) {
    splitConfigError.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

async function onCoverToJpgChange(event: Event): Promise<void> {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }

  const nextValue = target.checked;
  const prevValue = coverToJpg.value;
  coverToJpg.value = nextValue;
  saving.value = true;
  error.value = '';

  try {
    await setCoverToJpgSetting(nextValue);
  } catch (err) {
    coverToJpg.value = prevValue;
    target.checked = prevValue;
    error.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

async function onReadHistoryLimitChange(event: Event): Promise<void> {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }

  const nextValue = parseReadHistoryLimit(target.value);
  const prevValue = readHistoryLimit.value;
  if (nextValue === null) {
    target.value = String(prevValue);
    error.value = t('settings.readHistoryLimit.invalid');
    return;
  }

  readHistoryLimit.value = nextValue;
  saving.value = true;
  error.value = '';

  try {
    await setReadHistoryLimit(nextValue);
  } catch (err) {
    readHistoryLimit.value = prevValue;
    target.value = String(prevValue);
    error.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

async function loadVersion(): Promise<void> {
  try {
    version.value = await getServerVersion();
  } catch {
    version.value = '';
  }
}

onMounted(() => {
  void loadSettings();
  void loadVersion();
  void fetchShelves();
});
</script>

<style scoped>
.settings-page {
  display: grid;
  gap: 16px;
  max-width: 760px;
}

.settings-header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.settings-header h2 {
  margin: 0;
}

.settings-header p {
  color: #475569;
  margin: 6px 0 0;
}

.settings-message {
  margin: 0;
}

.settings-message-error {
  color: #b91c1c;
}

.settings-tabs {
  display: grid;
  gap: 16px;
}

.settings-tabs-list {
  border-bottom: 1px solid var(--border);
  display: flex;
  gap: 4px;
}

.settings-tab-trigger {
  background: transparent;
  border: 1px solid transparent;
  border-bottom: none;
  border-radius: 8px 8px 0 0;
  color: #475569;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: -1px;
  padding: 8px 14px;
}

.settings-tab-trigger:hover {
  color: #1e293b;
}

.settings-tab-trigger[data-state='active'] {
  background: var(--surface, #fff);
  border-color: var(--border);
  color: #1e293b;
}

.settings-tab-content {
  display: grid;
  gap: 16px;
}

.settings-group {
  display: grid;
  gap: 12px;
  padding: 16px;
}

.settings-group h3 {
  margin: 0;
}

.mobile-connect-link {
  justify-self: start;
  text-decoration: none;
}

.setting-item {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.setting-label {
  font-weight: 600;
}

.setting-value {
  color: #475569;
  font-family: monospace;
  font-size: 13px;
}

.setting-description {
  color: #475569;
  font-size: 13px;
  margin: 4px 0 0;
}


.setting-link {
  color: #2563eb;
  font-size: 13px;
  overflow-wrap: anywhere;
  text-align: right;
}

.font-license-section {
  border-top: 1px solid var(--border);
  display: grid;
  gap: 10px;
  padding-top: 12px;
}

.font-license-item {
  align-items: start;
  display: grid;
  gap: 3px;
}

.font-license-links {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 2px;
}


.setting-checkbox {
  cursor: pointer;
  height: 18px;
  width: 18px;
}

.setting-checkbox:disabled {
  cursor: not-allowed;
}

.setting-number {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  font: inherit;
  max-width: 120px;
  padding: 8px 10px;
}

.setting-number:disabled {
  cursor: not-allowed;
}

.setting-select {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  font: inherit;
  font-size: 13px;
  padding: 8px 10px;
}

.setting-select:disabled {
  cursor: not-allowed;
}

.setting-textarea {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  font: inherit;
  font-family: monospace;
  font-size: 13px;
  padding: 8px 10px;
  resize: vertical;
  width: 100%;
}

.setting-textarea:disabled {
  cursor: not-allowed;
}

.setting-item-stacked {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.split-config-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-start;
}

.shelves-table {
  border-collapse: collapse;
  font-size: 13px;
  width: 100%;
}

.shelves-table th {
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  padding: 4px 8px 6px;
  text-align: left;
  text-transform: uppercase;
}

.shelves-table td {
  border-top: 1px solid #e2e8f0;
  padding: 8px;
  vertical-align: middle;
}

.shelf-id-cell {
  color: #64748b;
  font-family: monospace;
  font-size: 12px;
}

.shelf-action-cell {
  text-align: right;
  white-space: nowrap;
}

.shelf-modify-btn {
  background: transparent;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #475569;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  margin-right: 6px;
  padding: 3px 10px;
}

.shelf-modify-btn:hover:not(:disabled) {
  background: #f1f5f9;
}

.shelf-modify-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-remove-btn {
  background: transparent;
  border: 1px solid #fca5a5;
  border-radius: 6px;
  color: #b91c1c;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 3px 10px;
}

.shelf-remove-btn:hover:not(:disabled) {
  background: #fef2f2;
}

.shelf-remove-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-modify-readonly-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.shelf-modify-label {
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.shelf-modify-value {
  font-size: 13px;
  word-break: break-all;
}


.shelf-add-row {
  display: flex;
}

.shelf-add-toggle {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 12px;
}

.shelf-add-toggle:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-add-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.shelf-add-fields {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.shelf-add-input {
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  padding: 7px 10px;
}

.shelf-add-input:disabled {
  cursor: not-allowed;
}

.shelf-add-help {
  color: #64748b;
  font-size: 12px;
  margin: -2px 0 0;
}

.shelf-add-dir-row {
  display: flex;
  gap: 6px;
}

.shelf-add-dir-input {
  flex: 1;
  min-width: 0;
}

.shelf-browse-btn {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  padding: 6px 10px;
  white-space: nowrap;
}

.shelf-browse-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.shelf-add-actions {
  display: flex;
  justify-content: flex-end;
}

.create-layer-submit {
  background: #2563eb;
  border: 1px solid #1d4ed8;
  border-radius: 6px;
  color: #ffffff;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 14px;
}

.create-layer-submit:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
