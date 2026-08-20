// @vitest-environment jsdom
import { createApp, h, nextTick, ref, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

// The illustration component reaches the asset API and the locale; neither is
// what this file is about. The stub records the props the slot was mounted with.
vi.mock('@/features/reader/components/ReaderAssetImage.vue', () => ({
  default: {
    props: ['bookId', 'sourceId', 'name', 'alt'],
    setup: (props: Record<string, string>) => () =>
      h('img', { 'data-name': props.name, alt: props.alt })
  }
}));

import ReaderSafeHtml from './ReaderSafeHtml.vue';
import { renderMarkdownBlocks, type ReaderMarkdownAsset } from '@/utils/renderMarkdownBlocks';

interface Mounted {
  app: App;
  host: HTMLElement;
  setBlock: (source: string) => Promise<void>;
}

let mounted: Mounted | null = null;

function block(source: string): { html: string; images: ReaderMarkdownAsset[] } {
  const [first] = renderMarkdownBlocks(source);
  return first ?? { html: '', images: [] };
}

async function mount(source: string): Promise<Mounted> {
  const current = ref(block(source));
  const host = document.createElement('div');
  document.body.append(host);

  const app = createApp({
    setup: () => () =>
      h(ReaderSafeHtml, {
        html: current.value.html,
        images: current.value.images,
        bookId: 'book-1',
        sourceId: 'source-1'
      })
  });
  app.mount(host);
  await nextTick();
  await nextTick();

  mounted = {
    app,
    host,
    setBlock: async (next: string) => {
      current.value = block(next);
      await nextTick();
      await nextTick();
    }
  };
  return mounted;
}

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('ReaderSafeHtml', () => {
  it('sanitizes with the reader profile inside the content element', async () => {
    const { host } = await mount('## Heading\n\n<script>alert(1)</script>\n\nText');
    const content = host.querySelector('.reader-safe-html > .reader-safe-html-content');

    expect(content).not.toBeNull();
    expect(content?.querySelector('h3')?.className).toBe('reader-md-h2');
    expect(host.querySelector('script')).toBeNull();
    expect(host.innerHTML).not.toContain('alert(1)');
  });

  it('mounts an illustration into its slot and drops the locator title', async () => {
    const { host } = await mount('Before\n\n![A map](assets/map.png)\n\nAfter');
    const slot = host.querySelector('span.reader-asset-slot');

    expect(slot).not.toBeNull();
    expect(slot?.hasAttribute('title')).toBe(false);
    expect(slot?.querySelector('img')?.getAttribute('data-name')).toBe('map.png');
    expect(slot?.querySelector('img')?.getAttribute('alt')).toBe('A map');
  });

  it('re-resolves its slots when the block changes', async () => {
    const { host, setBlock } = await mount('![A map](assets/map.png)');
    expect(host.querySelectorAll('img')).toHaveLength(1);

    await setBlock('No illustration here.');
    expect(host.querySelectorAll('img')).toHaveLength(0);

    await setBlock('![Cover](assets/cover.jpg)');
    const images = host.querySelectorAll('img');
    expect(images).toHaveLength(1);
    expect(images[0].getAttribute('data-name')).toBe('cover.jpg');
  });
});
