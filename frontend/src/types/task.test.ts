import { describe, expect, it } from 'vitest';
import { layerTransferCounts, type TaskChain } from './task';

function chainWithResult(result: unknown): TaskChain {
  return {
    id: 'chain-1',
    name: 'layer_transfer',
    title: 'Move a folder to another shelf',
    status: 'running',
    percentage: 50,
    tasks: [
      {
        name: 'layer_transfer',
        title: 'Move a folder to another shelf',
        status: 'running',
        percentage: 50,
        result
      }
    ]
  };
}

describe('layerTransferCounts', () => {
  it('returns null for a chain with no layer-transfer result yet', () => {
    expect(layerTransferCounts(null)).toBeNull();
    expect(layerTransferCounts(chainWithResult(undefined))).toBeNull();
  });

  it('counts processed books as succeeded plus failed', () => {
    const counts = layerTransferCounts(
      chainWithResult({
        operation: 'move',
        source_shelf: 'a',
        target_shelf: 'b',
        source_layer: ['Fiction'],
        target_layer: ['Fiction'],
        total: 5,
        succeeded_ids: ['book-1', 'book-2', 'book-3'],
        failures: [{ book_id: 'book-4', code: 'move_failed' }],
        layer_failures: 0
      })
    );

    expect(counts).toEqual({ processed: 4, total: 5, succeeded: 3, failed: 1 });
  });
});
