interface ReadingBlock {
  charStart: number;
  top: number;
}

export interface MeasuredReadingBlocks {
  blocks: ReadingBlock[];
  length: number;
  bottom: number;
}

function blockCharStarts(weights: readonly number[], length: number): number[] | null {
  const count = weights.length;
  if (count === 0 || length < count) return null;
  const safe = weights.map((weight) => (Number.isFinite(weight) && weight > 0 ? weight : 1));
  const total = safe.reduce((sum, weight) => sum + weight, 0);
  const starts: number[] = [];
  let before = 0;
  for (let i = 0; i < count; i += 1) {
    const proportional = Math.floor((length * before) / total);
    const afterPrevious = i === 0 ? 0 : starts[i - 1] + 1;
    starts.push(Math.min(Math.max(proportional, afterPrevious), length - (count - i)));
    before += safe[i];
  }
  return starts;
}

export function measureReadingBlocks(
  weights: readonly number[],
  tops: readonly number[],
  bottom: number,
  length: number
): MeasuredReadingBlocks | null {
  const starts = blockCharStarts(weights, length);
  if (!starts || tops.length !== starts.length) return null;
  const blocks = starts.map((charStart, i) => ({ charStart, top: i === 0 ? 0 : tops[i] }));
  const ends = [...blocks.slice(1).map((block) => block.top), bottom];
  if (blocks.some((block, i) => !(ends[i] > block.top))) return null;
  return { blocks, length, bottom };
}

function charEnd(measured: MeasuredReadingBlocks, i: number): number {
  return measured.blocks[i + 1]?.charStart ?? measured.length;
}

function pixelEnd(measured: MeasuredReadingBlocks, i: number): number {
  return measured.blocks[i + 1]?.top ?? measured.bottom;
}

export function offsetAtScroll(measured: MeasuredReadingBlocks, scrollTop: number): number {
  const { blocks } = measured;
  let i = 0;
  while (i + 1 < blocks.length && scrollTop >= blocks[i + 1].top) i += 1;
  const height = pixelEnd(measured, i) - blocks[i].top;
  const chars = charEnd(measured, i) - blocks[i].charStart;
  const within = Math.min(Math.max(0, scrollTop - blocks[i].top), height);
  return blocks[i].charStart + Math.floor((within * chars) / height);
}

export function scrollAtOffset(measured: MeasuredReadingBlocks, offset: number): number {
  const { blocks } = measured;
  const clamped = Math.min(Math.max(0, offset), measured.length);
  let j = 0;
  while (j + 1 < blocks.length && clamped >= blocks[j + 1].charStart) j += 1;
  const height = pixelEnd(measured, j) - blocks[j].top;
  const chars = charEnd(measured, j) - blocks[j].charStart;
  return blocks[j].top + Math.floor(((clamped - blocks[j].charStart) * height) / chars);
}
