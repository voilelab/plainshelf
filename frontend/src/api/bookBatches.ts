import { buildShelfApiPath, fetchJson, isMockApiMode } from './client';
import { mockStartBookBatch } from './mocks/books';
import type { BookBatchRequest } from '@/types/task';

interface BookBatchResponse {
  taskchain_id: string;
}

export async function startBookBatch(request: BookBatchRequest): Promise<string> {
  if (isMockApiMode()) {
    return mockStartBookBatch(request);
  }

  const response = await fetchJson<BookBatchResponse>(
    buildShelfApiPath('/book-batches'),
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request)
    },
    { acceptStatuses: [409] }
  );
  return response.taskchain_id;
}
