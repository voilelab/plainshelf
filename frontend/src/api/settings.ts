import { fetchJson, isMockApiMode } from './client';

interface SettingResponse {
  value?: unknown;
}

let mockCoverToJpg = false;
let mockReadHistoryLimit = 100;

function toBoolean(value: unknown): boolean {
  return value === true || value === 'true' || value === 1 || value === '1';
}

function toNonNegativeInteger(value: unknown, fallback = 0): number {
  if (typeof value === 'number' && Number.isInteger(value) && value >= 0) {
    return value;
  }

  if (typeof value === 'string') {
    const parsed = Number(value.trim());
    if (Number.isInteger(parsed) && parsed >= 0) {
      return parsed;
    }
  }

  return fallback;
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

export async function getReadHistoryLimitSetting(): Promise<number> {
  if (isMockApiMode()) {
    return mockReadHistoryLimit;
  }

  const res = await fetchJson<SettingResponse>('/api/setting/read_history_limit');
  return toNonNegativeInteger(res?.value);
}

export async function setReadHistoryLimitSetting(limit: number): Promise<void> {
  if (!Number.isInteger(limit) || limit < 0) {
    throw new Error('Read history limit must be a non-negative integer.');
  }

  if (isMockApiMode()) {
    mockReadHistoryLimit = limit;
    return;
  }

  await fetchJson<void>('/api/setting/read_history_limit', {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain;charset=UTF-8'
    },
    body: String(limit)
  });
}
