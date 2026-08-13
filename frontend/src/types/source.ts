import type { BookFormat, SplitConfig } from '@/types/book';

export interface SourceMeta {
  schema_version?: number;
  id: string;
  created_at: string;
  comment: string;
  md5_hash: string;
  /** Authoritative for schema-versioned sources. Missing means legacy. */
  format?: BookFormat;
  line_count?: number;
  char_count?: number;
  /** Kept intact while legacy sources remain readable. Ignored when format exists. */
  split_config?: SplitConfig;
}

export interface CreateSourceOptions {
  content?: string;
  format?: 'txt' | 'md';
  comment?: string;
  setCurrent?: boolean;
}
