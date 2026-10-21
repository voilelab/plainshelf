import { ref, type Ref } from 'vue';
import {
  DEFAULT_LOG_RETENTION_DAYS,
  getCoverToJpgSetting,
  getEpubImportStrategySetting,
  getLogRetentionDaysSetting,
  getShowNsfwSetting,
  setCoverToJpgSetting,
  setEpubImportStrategySetting,
  setLogRetentionDaysSetting,
  setShowNsfwSetting
} from '@/api/settings';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { getReadHistoryLimit, setReadHistoryLimit } from '@/storage/readHistory';
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
  showNsfw: Ref<boolean>;
  logRetentionDays: Ref<number>;
  readHistoryLimit: Ref<number>;
  readerLaunchMode: Ref<ReaderLaunchMode>;
  epubPreset: Ref<EpubImportPreset>;
  epubIncludeDescription: Ref<boolean>;
  epubImportError: Ref<string>;
  loadSettings: () => Promise<void>;
  onCoverToJpgChange: (value: boolean) => Promise<void>;
  onShowNsfwChange: (value: boolean) => Promise<void>;
  onLogRetentionDaysChange: (value: number) => Promise<void>;
  onReadHistoryLimitChange: (value: number) => Promise<void>;
  onReaderLaunchModeChange: (mode: ReaderLaunchMode) => void;
  onEpubPresetChange: (preset: EpubImportPreset) => void;
  onSaveEpubImportStrategy: () => Promise<void>;
}

export function useServerSettingsForm(options: {
  serverSettingsEditable: Ref<boolean>;
}): ServerSettingsForm {
  const { serverSettingsEditable } = options;
  const { t } = useI18n();

  const loading = ref(false);
  const saving = ref(false);
  const error = ref('');
  const coverToJpg = ref(false);
  const showNsfw = ref(false);
  const logRetentionDays = ref(DEFAULT_LOG_RETENTION_DAYS);
  const readHistoryLimit = ref(0);
  const readerLaunchMode = ref<ReaderLaunchMode>(getReaderLaunchMode());
  const epubPreset = ref<EpubImportPreset>(DEFAULT_EPUB_IMPORT_STRATEGY.preset);
  const epubIncludeDescription = ref(DEFAULT_EPUB_IMPORT_STRATEGY.include_description);
  const preservedEpubKeepImages = ref<boolean | undefined>(undefined);
  const epubImportError = ref('');

  async function loadSettings(): Promise<void> {
    loading.value = true;
    error.value = '';

    try {
      readHistoryLimit.value = await getReadHistoryLimit();
      readerLaunchMode.value = getReaderLaunchMode();
      if (!serverSettingsEditable.value) {
        return;
      }

      const [nextCoverToJpg, nextEpubStrategy, nextLogRetentionDays, nextShowNsfw] = await Promise.all([
        getCoverToJpgSetting(),
        getEpubImportStrategySetting(),
        getLogRetentionDaysSetting(),
        getShowNsfwSetting()
      ]);
      coverToJpg.value = nextCoverToJpg;
      logRetentionDays.value = nextLogRetentionDays;
      showNsfw.value = nextShowNsfw;
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
    preservedEpubKeepImages.value = strategy.keep_images;
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
        ...(preservedEpubKeepImages.value === undefined ? {} : { keep_images: preservedEpubKeepImages.value })
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
      coverToJpg.value = prevValue;
      error.value = err instanceof Error ? err.message : t('settings.saveFailed');
    } finally {
      saving.value = false;
    }
  }

  async function onShowNsfwChange(nextValue: boolean): Promise<void> {
    const prevValue = showNsfw.value;
    showNsfw.value = nextValue;
    saving.value = true;
    error.value = '';

    try {
      await setShowNsfwSetting(nextValue);
    } catch (err) {
      showNsfw.value = prevValue;
      error.value = err instanceof Error ? err.message : t('settings.saveFailed');
      saving.value = false;
      return;
    }

    await refreshBookAndFolderListings();
    saving.value = false;
  }

  async function refreshBookAndFolderListings(): Promise<void> {
    const { fetchBooks } = useBookStore();
    const { fetchFolders } = useFolderStore();
    await Promise.all([fetchBooks(), fetchFolders()]);
  }

  async function onLogRetentionDaysChange(value: number): Promise<void> {
    const nextValue = parseLogRetentionDays(String(value));
    const prevValue = logRetentionDays.value;
    if (nextValue === null) {
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
      error.value = err instanceof Error ? err.message : t('settings.saveFailed');
    } finally {
      saving.value = false;
    }
  }

  async function onReadHistoryLimitChange(value: number): Promise<void> {
    const nextValue = parseReadHistoryLimit(String(value));
    const prevValue = readHistoryLimit.value;
    if (nextValue === null) {
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
      error.value = err instanceof Error ? err.message : t('settings.saveFailed');
    } finally {
      saving.value = false;
    }
  }

  function onReaderLaunchModeChange(mode: ReaderLaunchMode): void {
    setReaderLaunchMode(mode);
    readerLaunchMode.value = getReaderLaunchMode();
    error.value = '';
  }

  return {
    loading,
    saving,
    error,
    coverToJpg,
    showNsfw,
    logRetentionDays,
    readHistoryLimit,
    readerLaunchMode,
    epubPreset,
    epubIncludeDescription,
    epubImportError,
    loadSettings,
    onCoverToJpgChange,
    onShowNsfwChange,
    onLogRetentionDaysChange,
    onReadHistoryLimitChange,
    onReaderLaunchModeChange,
    onEpubPresetChange,
    onSaveEpubImportStrategy
  };
}
