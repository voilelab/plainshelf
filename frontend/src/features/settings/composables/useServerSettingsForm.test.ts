// @vitest-environment jsdom
import { ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const settingsApi = vi.hoisted(() => ({
  getCoverToJpgSetting: vi.fn(async () => false),
  getEpubImportStrategySetting: vi.fn(async () => ({ preset: 'markdown', include_description: true })),
  getLogRetentionDaysSetting: vi.fn(async () => 30),
  getShowNsfwSetting: vi.fn(async () => false),
  setCoverToJpgSetting: vi.fn(async () => {}),
  setEpubImportStrategySetting: vi.fn(async () => {}),
  setLogRetentionDaysSetting: vi.fn(async () => {}),
  setShowNsfwSetting: vi.fn(async () => {})
}));

vi.mock('@/api/settings', () => ({
  DEFAULT_LOG_RETENTION_DAYS: 30,
  ...settingsApi
}));

const stores = vi.hoisted(() => ({
  fetchBooks: vi.fn(async () => {}),
  fetchFolders: vi.fn(async () => {})
}));

vi.mock('@/composables/useBookStore', () => ({
  useBookStore: () => ({ fetchBooks: stores.fetchBooks })
}));
vi.mock('@/composables/useFolderStore', () => ({
  useFolderStore: () => ({ fetchFolders: stores.fetchFolders })
}));

vi.mock('@/storage/readHistory', () => ({
  getReadHistoryLimit: vi.fn(async () => 50),
  setReadHistoryLimit: vi.fn(async () => {})
}));

const { useServerSettingsForm } = await import('./useServerSettingsForm');

function form() {
  return useServerSettingsForm({ serverSettingsEditable: ref(true) });
}

beforeEach(() => {
  settingsApi.getShowNsfwSetting.mockResolvedValue(false);
  settingsApi.setShowNsfwSetting.mockResolvedValue(undefined);
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('useServerSettingsForm show_nsfw', () => {
  it('loads the stored value along with the other server settings', async () => {
    settingsApi.getShowNsfwSetting.mockResolvedValue(true);

    const { showNsfw, loadSettings } = form();
    await loadSettings();

    expect(settingsApi.getShowNsfwSetting).toHaveBeenCalledTimes(1);
    expect(showNsfw.value).toBe(true);
  });

  it('stores the new value and refetches the shelf the setting changed', async () => {
    const { showNsfw, onShowNsfwChange, error } = form();

    await onShowNsfwChange(true);

    expect(settingsApi.setShowNsfwSetting).toHaveBeenCalledWith(true);
    // Both stores are module-level singletons that outlive this page: without
    // this the sidebar and a library page returned to would still be showing
    // the shelf the previous value produced.
    expect(stores.fetchBooks).toHaveBeenCalledTimes(1);
    expect(stores.fetchFolders).toHaveBeenCalledTimes(1);
    expect(showNsfw.value).toBe(true);
    expect(error.value).toBe('');
  });

  it('rolls the switch back and skips the refetch when the save fails', async () => {
    settingsApi.setShowNsfwSetting.mockRejectedValue(new Error('nope'));

    const { showNsfw, onShowNsfwChange, error, saving } = form();
    await onShowNsfwChange(true);

    expect(showNsfw.value).toBe(false);
    expect(error.value).toBe('nope');
    // Nothing changed on the server, so refetching would only redraw the same
    // listing — and would clear an error the sidebar is showing for its own
    // reasons.
    expect(stores.fetchBooks).not.toHaveBeenCalled();
    expect(stores.fetchFolders).not.toHaveBeenCalled();
    expect(saving.value).toBe(false);
  });
});
