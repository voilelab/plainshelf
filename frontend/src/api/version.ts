import { fetchJson, isMockApiMode } from './client';

interface VersionResponse {
  version?: unknown;
}

export async function getServerVersion(): Promise<string> {
  if (isMockApiMode()) {
    return 'dev';
  }

  const res = await fetchJson<VersionResponse>('/api/version');
  return typeof res?.version === 'string' ? res.version : '';
}
