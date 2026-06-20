import { fetchJson, isMockApiMode } from './client';

interface ModeResponse {
  read_only?: unknown;
}

function toBoolean(value: unknown): boolean {
  return value === true || value === 'true' || value === 1 || value === '1';
}

export async function getReadOnlyMode(): Promise<boolean> {
  if (isMockApiMode()) {
    return false;
  }

  const res = await fetchJson<ModeResponse>('/api/mode');
  return toBoolean(res?.read_only);
}
