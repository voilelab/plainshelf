import { describe, expect, it } from 'vitest';
import vectors from './folderOrder.vectors.json';
import { planFolderOrder, type FolderOrderPlacement } from './folderOrder';
import type { FolderMetadata } from './folders';

interface FolderOrderVector {
  keys: (number | null)[];
  source: number;
  target: number;
  side: FolderOrderPlacement;
  plan: [string, number][];
}

const cases = vectors as FolderOrderVector[];

describe('planFolderOrder matches the Lean model', () => {
  it('covers every move among up to four siblings in each key layout', () => {
    expect(cases.length).toBe(406);
  });

  it.each(cases)('keys $keys, f$source onto f$target $side', (vector) => {
    const siblings = vector.keys.map((_key, i) => `f${i}`);
    const metadata: FolderMetadata = Object.fromEntries(
      vector.keys.map((key, i) => [
        `f${i}`,
        key === null ? { version: `v${i}` } : { sort_key: key, version: `v${i}` }
      ])
    );
    const plan = planFolderOrder(
      siblings,
      `f${vector.source}`,
      `f${vector.target}`,
      metadata,
      vector.side
    );
    expect(plan.map(({ path, key }) => [path, key])).toEqual(vector.plan);
    for (const step of plan) {
      expect(step.version).toBe(`v${step.path.slice(1)}`);
    }
  });
});
