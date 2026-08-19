import { ref, type Ref } from 'vue';
import {
  getCoverToJpgSetting,
  getEpubImportStrategySetting,
  setCoverToJpgSetting,
  setEpubImportStrategySetting
} from '@/api/settings';
// Reading history and its retention limit are per-device state, not server settings.
import { getReadHistoryLimit, setReadHistoryLimit } from '@/storage/readHistory';
import {
  DEFAULT_EPUB_IMPORT_STRATEGY,
  type EpubImportPreset,
  type EpubImportStrategy,
} from '@/types/book';
import { useI18n } from '@/i18n';
import { normalizeEpubImportPreset } from '@/utils/epubStrategy';
import {
  parseReadHistoryLimit
} from '@/features/settings/utils/settingsDraft';

export interface ServerSettingsForm {
  loading: Ref<boolean>;
  saving: Ref<boolean>;
  error: Ref<string>;
  coverToJpg: Ref<boolean>;
  readHistoryLimit: Ref<number>;
  epubPreset: Ref<EpubImportPreset>;
  epubIncludeDescription: Ref<boolean>;
  epubImportError: Ref<string>;
  loadSettings: () => Promise<void>;
  onCoverToJpgChange: (event: Event) => Promise<void>;
  onReadHistoryLimitChange: (event: Event) => Promise<void>;
  onEpubPresetChange: (event: Event) => void;
  onSaveEpubImportStrategy: () => Promise<void>;
}

/**
 * The settings that live on the server (plus the device-local read-history
 * limit, which loads on every client) and the shared load/save state the
 * settings page uses to disable its controls while a request is in flight.
 *
 * The change handlers take the raw DOM event because several of them restore
 * the control's own value when a save fails, which needs the element itself.
 */
export function useServerSettingsForm(options: {
  serverSettingsEditable: Ref<boolean>;
}): ServerSettingsForm {
  const { serverSettingsEditable } = options;
  const { t } = useI18n();

  const loading = ref(false);
  const saving = ref(false);
  const error = ref('');
  const coverToJpg = ref(false);
  const readHistoryLimit = ref(0);
  const epubPreset = ref<EpubImportPreset>(DEFAULT_EPUB_IMPORT_STRATEGY.preset);
  const epubIncludeDescription = ref(DEFAULT_EPUB_IMPORT_STRATEGY.include_description);
  // Held only so saving another EPUB setting does not erase it. There is no
  // control for it yet; it is configured through the API or the config file.
  const epubKeepImages = ref<boolean | undefined>(undefined);
  const epubImportError = ref('');

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

      const [nextCoverToJpg, nextEpubStrategy] = await Promise.all([
        getCoverToJpgSetting(),
        getEpubImportStrategySetting()
      ]);
      coverToJpg.value = nextCoverToJpg;
      hydrateEpubImportDraft(nextEpubStrategy);
    } catch (err) {
      error.value = err instanceof Error ? err.message : t('settings.loadFailed');
    } finally {
      loading.value = false;
    }
  }

  function hydrateEpubImportDraft(strategy: EpubImportStrategy): void {
    epubPreset.value = strategy.preset;
    epubIncludeDescription.value = strategy.include_description;
    epubKeepImages.value = strategy.keep_images;
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
        include_description: epubIncludeDescription.value,
        ...(epubKeepImages.value === undefined ? {} : { keep_images: epubKeepImages.value })
      });
    } catch (err) {
      epubImportError.value = err instanceof Error ? err.message : t('settings.saveFailed');
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

  return {
    loading,
    saving,
    error,
    coverToJpg,
    readHistoryLimit,
    epubPreset,
    epubIncludeDescription,
    epubImportError,
    loadSettings,
    onCoverToJpgChange,
    onReadHistoryLimitChange,
    onEpubPresetChange,
    onSaveEpubImportStrategy
  };
}
