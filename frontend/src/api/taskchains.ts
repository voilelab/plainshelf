import { fetchJson, isMockApiMode } from './client';
import type { TaskChain } from '@/types/task';
import { mockGetTaskChain } from './mocks/taskchains';

export async function getTaskChain(taskChainID: string): Promise<TaskChain> {
  if (isMockApiMode()) {
    return mockGetTaskChain(taskChainID);
  }

  return await fetchJson<TaskChain>(`/api/taskchains/${encodeURIComponent(taskChainID)}`);
}
