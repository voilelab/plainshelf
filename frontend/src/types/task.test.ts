import { describe, expect, it } from 'vitest';
import { folderTransferCounts, type TaskChain } from './task';

function chainWithResult(result: unknown): TaskChain {
  return {
    id: 'chain-1',
    name: 'folder_transfer',
    title: 'Move a folder to another shelf',
    status: 'running',
    percentage: 50,
    tasks: [
      {
        name: 'folder_transfer',
        title: 'Move a folder to another shelf',
        status: 'running',
        percentage: 50,
        result
      }
    ]
  };
}

describe('folderTransferCounts', () => {
  it('returns null for a chain with no folder-transfer result yet', () => {
    expect(folderTransferCounts(null)).toBeNull();
    expect(folderTransferCounts(chainWithResult(undefined))).toBeNull();
  });

  it('counts processed books as succeeded plus failed', () => {
    const counts = folderTransferCounts(
      chainWithResult({
        operation: 'move',
        source_shelf: 'a',
        target_shelf: 'b',
        source_folder: ['Fiction'],
        target_folder: ['Fiction'],
        total: 5,
        succeeded_ids: ['book-1', 'book-2', 'book-3'],
        failures: [{ book_id: 'book-4', code: 'move_failed' }],
        folder_failures: 0
      })
    );

    expect(counts).toEqual({ processed: 4, total: 5, succeeded: 3, failed: 1 });
  });
});
