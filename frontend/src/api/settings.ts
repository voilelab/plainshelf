import type { SplitConfig } from '@/types/book';
import { normalizeSplitConfig, buildSplitConfigPayload } from '@/utils/splitConfig';
import { fetchJson, isMockApiMode } from './client';

interface SettingResponse {
  value?: unknown;
}

let mockCoverToJpg = false;
let mockDefaultSplitConfig: SplitConfig = { type: 'none' };

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

export async function getDefaultSplitConfigSetting(): Promise<SplitConfig> {
  if (isMockApiMode()) {
    return mockDefaultSplitConfig;
  }

  const res = await fetchJson<SettingResponse>('/api/setting/default_split_config');
  return normalizeSplitConfig(res?.value);
}

export async function setDefaultSplitConfigSetting(config: SplitConfig): Promise<void> {
  const payload = buildSplitConfigPayload(config);

  if (isMockApiMode()) {
    mockDefaultSplitConfig = normalizeSplitConfig(payload);
    return;
  }

  await fetchJson<void>('/api/setting/default_split_config', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });
}
