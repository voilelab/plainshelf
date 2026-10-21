import { ApiError } from '@/api/client';
import type { TaskChain, TaskStatus } from '@/types/task';

interface MockTaskChain extends TaskChain {
  advance: () => void;
}

const mockTaskChains = new Map<string, MockTaskChain>();

let mockTaskChainCounter = 0;

function nextMockTaskChainID(): string {
  mockTaskChainCounter += 1;
  return `mock-taskchain-${mockTaskChainCounter}`;
}

export function registerMockTaskChain(options: {
  name: string;
  title: string;
  total: number;
  finalStatus?: TaskStatus;
  onItem?: (index: number) => void;
  getResult?: () => unknown;
}): string {
  const { name, title, total, finalStatus = 'completed', onItem, getResult } = options;
  const id = nextMockTaskChainID();

  let done = 0;
  const chain: MockTaskChain = {
    id,
    name,
    title,
    description: `0 of ${total}`,
    status: total === 0 ? finalStatus : 'pending',
    percentage: total === 0 ? 100 : 0,
    created_at: new Date().toISOString(),
    tasks: [
      {
        name,
        title,
        description: `0 of ${total}`,
        status: total === 0 ? finalStatus : 'pending',
        percentage: total === 0 ? 100 : 0
      }
    ],
    advance() {
      if (done >= total) {
        return;
      }

      onItem?.(done);
      done += 1;

      const percentage = (done / total) * 100;
      const status: TaskStatus = done >= total ? finalStatus : 'running';
      const description = `${done} of ${total}`;

      chain.percentage = percentage;
      chain.status = status;
      chain.description = description;
      chain.tasks[0] = { ...chain.tasks[0], percentage, status, description, result: getResult?.() };
    }
  };

  mockTaskChains.set(id, chain);
  return id;
}

function stripMockInternals(chain: MockTaskChain): TaskChain {
  const { advance: _advance, ...rest } = chain;
  return { ...rest, tasks: rest.tasks.map((task) => ({ ...task })) };
}

export function mockGetTaskChain(taskChainID: string): TaskChain {
  const chain = mockTaskChains.get(taskChainID);
  if (!chain) {
    throw new ApiError('Task chain not found.', { status: 404 });
  }

  chain.advance();
  return stripMockInternals(chain);
}
