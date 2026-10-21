import { describe, expect, it } from 'vitest';
import type { DownloadState } from '@/types/book';
import {
  DOWNLOAD_REQUIRED_QUERY_KEY,
  isReadableWhenDownloadRequired
} from './readerDownloadGate';

describe('isReadableWhenDownloadRequired', () => {
  it('allows a downloaded book to be read', () => {
    expect(isReadableWhenDownloadRequired('downloaded')).toBe(true);
  });

  it('allows a downloaded-but-stale book to be read', () => {
    expect(isReadableWhenDownloadRequired('update_available')).toBe(true);
  });

  it.each<DownloadState>(['not_downloaded', 'downloading', 'failed'])(
    'refuses reading while %s',
    (state) => {
      expect(isReadableWhenDownloadRequired(state)).toBe(false);
    }
  );

  it('matches the query flag BookDetailPage reads', () => {
    expect(DOWNLOAD_REQUIRED_QUERY_KEY).toBe('downloadRequired');
  });
});
