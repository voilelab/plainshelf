import { DEFAULT_EPUB_IMPORT_STRATEGY, type EpubImportStrategy } from '@/types/book';
import { normalizeEpubImportStrategy } from '@/utils/epubStrategy';
import { fetchJson, isMockApiMode } from './client';

interface SettingResponse {
  value?: unknown;
}

export const DEFAULT_LOG_RETENTION_DAYS = 30;

let mockCoverToJpg = false;
let mockShowNsfw = false;
let mockLogRetentionDays = DEFAULT_LOG_RETENTION_DAYS;
let mockEpubImportStrategy: EpubImportStrategy = { ...DEFAULT_EPUB_IMPORT_STRATEGY };

function toBoolean(value: unknown): boolean {
  return value === true || value === 'true' || value === 1 || value === '1';
}

export async function getCoverToJpgSetting(): Promise<boolean> {
  if (isMockApiMode()) {
    return mockCoverToJpg;
  }

  const res = await fetchJson<SettingResponse>('/api/setting/cover_to_jpg');
  return toBoolean(res?.value);
}

export async function setCoverToJpgSetting(enabled: boolean): Promise<void> {
  if (isMockApiMode()) {
    mockCoverToJpg = enabled;
    return;
  }

  await fetchJson<void>('/api/setting/cover_to_jpg', {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain;charset=UTF-8'
    },
    body: enabled ? 'true' : 'false'
  });
}

export async function getShowNsfwSetting(): Promise<boolean> {
  if (isMockApiMode()) {
    return mockShowNsfw;
  }

  const res = await fetchJson<SettingResponse>('/api/setting/show_nsfw');
  return toBoolean(res?.value);
}

export async function setShowNsfwSetting(enabled: boolean): Promise<void> {
  if (isMockApiMode()) {
    mockShowNsfw = enabled;
    return;
  }

  await fetchJson<void>('/api/setting/show_nsfw', {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain;charset=UTF-8'
    },
    body: enabled ? 'true' : 'false'
  });
}

export async function getEpubImportStrategySetting(): Promise<EpubImportStrategy> {
  if (isMockApiMode()) {
    return mockEpubImportStrategy;
  }

  const res = await fetchJson<SettingResponse>('/api/setting/epub_import_strategy');
  return normalizeEpubImportStrategy(res?.value);
}

export async function setEpubImportStrategySetting(strategy: EpubImportStrategy): Promise<void> {
  const payload = normalizeEpubImportStrategy(strategy);

  if (isMockApiMode()) {
    mockEpubImportStrategy = payload;
    return;
  }

  await fetchJson<void>('/api/setting/epub_import_strategy', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });
}

export async function getLogRetentionDaysSetting(): Promise<number> {
  if (isMockApiMode()) {
    return mockLogRetentionDays;
  }

  const res = await fetchJson<SettingResponse>('/api/setting/log_retention_days');
  const value = Number(res?.value);
  return Number.isInteger(value) && value >= 0 ? value : DEFAULT_LOG_RETENTION_DAYS;
}

export async function setLogRetentionDaysSetting(days: number): Promise<void> {
  if (isMockApiMode()) {
    mockLogRetentionDays = days;
    return;
  }

  await fetchJson<void>('/api/setting/log_retention_days', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(days)
  });
}
