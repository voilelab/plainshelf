import type { SplitConfig, SplitType } from '../types/book';

export function normalizeSplitType(value: unknown): SplitType {
  if (value === 'none' || value === 'line_count' || value === 'regex' || value === 'boundary') {
    return value;
  }
  return 'none';
}

export function normalizeSplitConfig(raw: unknown): SplitConfig {
  if (!raw || typeof raw !== 'object') {
    return { type: 'none' };
  }

  const data = raw as Record<string, unknown>;
  const type = normalizeSplitType(data.type);
  const normalized: SplitConfig = { type };

  if (typeof data.line_count === 'number' && Number.isFinite(data.line_count)) {
    normalized.line_count = Math.trunc(data.line_count);
  }

  if (typeof data.regex === 'string') {
    normalized.regex = data.regex;
  }

  if (Array.isArray(data.boundaries)) {
    normalized.boundaries = data.boundaries
      .filter((item): item is number => typeof item === 'number' && Number.isFinite(item))
      .map((item) => Math.trunc(item));
  }

  return normalized;
}

export function buildSplitConfigPayload(config: SplitConfig): SplitConfig {
  const type = normalizeSplitType(config.type);

  if (type === 'line_count') {
    return {
      type,
      line_count: Math.trunc(config.line_count ?? 0)
    };
  }

  if (type === 'regex') {
    return {
      type,
      regex: String(config.regex ?? '')
    };
  }

  if (type === 'boundary') {
    return {
      type,
      boundaries: (config.boundaries ?? [])
        .filter((item) => Number.isFinite(item))
        .map((item) => Math.trunc(item))
    };
  }

  return { type: 'none' };
}
