// @vitest-environment jsdom
import { createApp, h } from 'vue';
import { describe, expect, it } from 'vitest';

import FolderIcon from './FolderIcon.vue';

function render(icon?: string): string {
  const host = document.createElement('div');
  const app = createApp({ render: () => h(FolderIcon, { icon }) });
  app.mount(host);
  const html = host.innerHTML;
  app.unmount();
  return html;
}

describe('FolderIcon', () => {
  it('draws the named icon', () => {
    expect(render('rocket')).not.toBe(render(''));
  });

  // The server already maps an unknown name to '', but a listing from an older
  // server, or the mock backend, can still hand one over.
  it('draws the default for no icon and for a name this build does not ship', () => {
    const fallback = render(undefined);
    expect(render('')).toBe(fallback);
    expect(render('icon-from-a-newer-build')).toBe(fallback);
    expect(render('constructor')).toBe(fallback);
    expect(render('folder')).toBe(fallback);
  });

  it('is decoration for assistive technology', () => {
    expect(render('rocket')).toContain('aria-hidden="true"');
  });
});
