import type { BookFormat } from '@/types/book';

export interface SourceMeta {
  schema_version?: number;
  id: string;
  created_at: string;
  comment: string;
  md5_hash: string;
  format?: BookFormat;
  line_count?: number;
  char_count?: number;
  page_direction?: 'ltr' | 'rtl';
  page_list?: SourcePageList;
}

export interface CreateSourceOptions {
  content?: string;
  format?: 'txt' | 'md';
  comment?: string;
  setCurrent?: boolean;
}

export interface SourcePage {
  name: string;
  missing: boolean;
}

export interface SourcePageList {
  pages: SourcePage[];
  unlisted: string[];
}
