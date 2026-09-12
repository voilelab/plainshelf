// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import type { DownloadState } from '@/types/book';
import BookDownloadBadge from './BookDownloadBadge.vue';
import { mount } from '#testing/mount';


function mountBadge(state?: DownloadState): HTMLElement {
  return mount(BookDownloadBadge, { props: { state } }).host;
}

describe('BookDownloadBadge', () => {
  it('renders nothing when the book carries no download state', () => {
    // A server or desktop listing has no download concept; it must not gain an
    // empty marker.
    expect(mountBadge(undefined).querySelector('.book-download-badge')).toBeNull();
  });

  it('labels each state with distinct text, not colour alone', () => {
    const labels = (['not_downloaded', 'downloaded', 'update_available'] as const).map((state) => {
      const badge = mountBadge(state).querySelector('.book-download-badge');
      return badge?.querySelector('.book-download-text')?.textContent?.trim();
    });

    expect(labels).toEqual(['Not downloaded', 'Downloaded', 'Update available']);
    expect(new Set(labels).size).toBe(labels.length);
  });

  it('exposes the raw state for styling and assertions', () => {
    const badge = mountBadge('failed').querySelector('.book-download-badge');
    expect(badge?.getAttribute('data-download-state')).toBe('failed');
    expect(badge?.classList.contains('is-failed')).toBe(true);
  });
});
