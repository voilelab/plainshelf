import type { BookFormat } from '@/types/book';

export interface SourceMeta {
  schema_version?: number;
  id: string;
  created_at: string;
  comment: string;
  md5_hash: string;
  /** Authoritative for every source this build writes. */
  format?: BookFormat;
  line_count?: number;
  char_count?: number;
}

export interface CreateSourceOptions {
  content?: string;
  format?: 'txt' | 'md';
  comment?: string;
  setCurrent?: boolean;
}
