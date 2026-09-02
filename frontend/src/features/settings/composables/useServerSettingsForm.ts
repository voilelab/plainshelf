import { ref, type Ref } from 'vue';
import {
  DEFAULT_LOG_RETENTION_DAYS,
  getCoverToJpgSetting,
  getEpubImportStrategySetting,
  getLogRetentionDaysSetting,
  setCoverToJpgSetting,
  setEpubImportStrategySetting,
  setLogRetentionDaysSetting
} from '@/api/settings';
// Reading history and its retention limit are per-device state, not server settings.
import { getReadHistoryLimit, setReadHistoryLimit } from '@/storage/readHistory';
// The reader-launch preference is likewise per-device, not a server setting.
import {
  getReaderLaunchMode,
  setReaderLaunchMode,
  type ReaderLaunchMode
} from '@/composables/useReaderLaunchPreference';
import {
  DEFAULT_EPUB_IMPORT_STRATEGY,
  type EpubImportPreset,
  type EpubImportStrategy,
} from '@/types/book';
import { useI18n } from '@/i18n';
import { normalizeEpubImportPreset } from '@/utils/epubStrategy';
import {
  parseLogRetentionDays,
  parseReadHistoryLimit
} from '@/features/settings/utils/settingsDraft';

interface ServerSettingsForm {
  loading: Ref<boolean>;
  saving: Ref<boolean>;
  error: Ref<string>;
  coverToJpg: Ref<boolean>;
  logRetentionDays: Ref<number>;
  readHistoryLimit: Ref<number>;
  readerLaunchMode: Ref<ReaderLaunchMode>;
  epubPreset: Ref<EpubImportPreset>;
  epubIncludeDescription: Ref<boolean>;
  epubImportError: Ref<string>;
  loadSettings: () => Promise<void>;
  onCoverToJpgChange: (value: boolean) => Promise<void>;
  onLogRetentionDaysChange: (event: Event) => Promise<void>;
  onReadHistoryLimitChange: (event: Event) => Promise<void>;
  onReaderLaunchModeChange: (mode: ReaderLaunchMode) => void;
  onEpubPresetChange: (preset: EpubImportPreset) => void;
  onSaveEpubImportStrategy: () => Promise<void>;
}

/**
 * The settings that live on the server (plus the device-local read-history
 * limit, which loads on every client) and the shared load/save state the
 * settings page uses to disable its controls while a request is in flight.
 *
 * The select and number handlers take the raw DOM event because they restore
 * the control's own value when a save fails, which needs the element itself.
 * The cover switch renders from its ref, so that one takes the value.
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
  const logRetentionDays = ref(DEFAULT_LOG_RETENTION_DAYS);
  const readHistoryLimit = ref(0);
  const readerLaunchMode = ref<ReaderLaunchMode>(getReaderLaunchMode());
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
      // The reader-launch preference is device-local too and available on every
      // client, so re-read it here in case it changed in another tab.
      readerLaunchMode.value = getReaderLaunchMode();
      if (!serverSettingsEditable.value) {
        return;
      }

      const [nextCoverToJpg, nextEpubStrategy, nextLogRetentionDays] = await Promise.all([
        getCoverToJpgSetting(),
        getEpubImportStrategySetting(),
        getLogRetentionDaysSetting()
      ]);
      coverToJpg.value = nextCoverToJpg;
      logRetentionDays.value = nextLogRetentionDays;
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

  function onEpubPresetChange(preset: EpubImportPreset): void {
    epubPreset.value = normalizeEpubImportPreset(preset);
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

  async function onCoverToJpgChange(nextValue: boolean): Promise<void> {
    const prevValue = coverToJpg.value;
    coverToJpg.value = nextValue;
    saving.value = true;
    error.value = '';

    try {
      await setCoverToJpgSetting(nextValue);
    } catch (err) {
      // The switch renders from this ref, so putting it back is the whole
      // rollback — the native checkbox it replaced also needed its own DOM
      // state restored, because the browser had already toggled it.
      coverToJpg.value = prevValue;
      error.value = err instanceof Error ? err.message : t('settings.saveFailed');
    } finally {
      saving.value = false;
    }
  }

  async function onLogRetentionDaysChange(event: Event): Promise<void> {
    const target = event.target;
    if (!(target instanceof HTMLInputElement)) {
      return;
    }

    const nextValue = parseLogRetentionDays(target.value);
    const prevValue = logRetentionDays.value;
    if (nextValue === null) {
      target.value = String(prevValue);
      error.value = t('settings.logRetention.invalid');
      return;
    }

    logRetentionDays.value = nextValue;
    saving.value = true;
    error.value = '';

    try {
      await setLogRetentionDaysSetting(nextValue);
    } catch (err) {
      logRetentionDays.value = prevValue;
      target.value = String(prevValue);
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

  function onReaderLaunchModeChange(mode: ReaderLaunchMode): void {
    // Device-local and synchronous: setReaderLaunchMode persists to localStorage
    // (mirroring useAppZoom) with no server round-trip that could fail. The
    // setter coerces any unexpected option back to the default, so mirror the
    // stored result onto the local ref rather than the raw value.
    setReaderLaunchMode(mode);
    readerLaunchMode.value = getReaderLaunchMode();
    error.value = '';
  }

  return {
    loading,
    saving,
    error,
    coverToJpg,
    logRetentionDays,
    readHistoryLimit,
    readerLaunchMode,
    epubPreset,
    epubIncludeDescription,
    epubImportError,
    loadSettings,
    onCoverToJpgChange,
    onLogRetentionDaysChange,
    onReadHistoryLimitChange,
    onReaderLaunchModeChange,
    onEpubPresetChange,
    onSaveEpubImportStrategy
  };
}
