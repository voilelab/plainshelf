// @vitest-environment jsdom
import { createApp, defineComponent, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { SimilarBookPair, SimilarRelation } from '@/api/books';
import type { Book } from '@/types/book';

const mocks = vi.hoisted(() => ({
  deleteBook: vi.fn(),
  push: vi.fn()
}));

vi.mock('@/providers', () => ({
  bookshelfWriter: () => ({ deleteBook: mocks.deleteBook })
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push })
}));

// The real DeleteModal renders through a Reka portal, out of the host subtree.
// Stub it so its props (the comparison the card feeds it) are inspectable and
// its confirm is clickable without standing up the whole dialog stack.
vi.mock('@/components/DeleteModal.vue', () => ({
  default: defineComponent({
    props: {
      open: { type: Boolean, default: false },
      itemName: { type: String, default: '' },
      description: { type: String, default: '' },
      busy: { type: Boolean, default: false },
      error: { type: String, default: '' }
    },
    emits: ['cancel', 'confirm'],
    setup(props, { emit }) {
      return () =>
        props.open
          ? h('div', { class: 'delete-modal-stub' }, [
              h('p', { class: 'dm-item' }, props.itemName),
              h('p', { class: 'dm-desc' }, props.description),
              h('button', { class: 'dm-confirm', onClick: () => emit('confirm') }, 'confirm')
            ])
          : null;
    }
  })
}));

import SimilarPairCard from './SimilarPairCard.vue';
import { setLocale } from '@/i18n';

function book(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
    title: id,
    authors: [],
    tags: [],
    layers: [],
    format: 'txt',
    ...overrides
  };
}

function pair(overrides: Partial<SimilarBookPair> = {}): SimilarBookPair {
  const relation: SimilarRelation = overrides.relation ?? 'near_identical';
  return {
    a: 'book-a',
    b: 'book-b',
    jaccard: 0.85,
    containment_a: 0.9,
    containment_b: 0.9,
    norm_chars_a: 1000,
    norm_chars_b: 1000,
    relation,
    ...overrides
  };
}

let mounted: { app: App; host: HTMLElement } | null = null;

type CardProps = InstanceType<typeof SimilarPairCard>['$props'];

function mount(props: CardProps): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(SimilarPairCard, props) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function bookRows(host: HTMLElement): HTMLElement[] {
  return [...host.querySelectorAll<HTMLElement>('.similar-book-row')];
}

beforeEach(() => {
  setLocale('en');
  mocks.deleteBook.mockReset().mockResolvedValue(undefined);
  mocks.push.mockReset();
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('SimilarPairCard', () => {
  it('puts the fuller copy on top with the "more complete" badge for a subset', async () => {
    const host = mount({
      pair: pair({ relation: 'subset', jaccard: 0.758, norm_chars_a: 750, norm_chars_b: 1000 }),
      bookA: book('book-a', { title: 'Trimmed', char_count: 750 }),
      bookB: book('book-b', { title: 'Full', char_count: 1000 })
    });
    await flush();

    const rows = bookRows(host);
    expect(rows).toHaveLength(2);
    // B has more normalized characters, so it leads and wears the badge.
    expect(rows[0].querySelector('.similar-title')?.textContent).toBe('Full');
    expect(rows[0].querySelector('.similar-badge')).not.toBeNull();
    expect(rows[1].querySelector('.similar-badge')).toBeNull();
  });

  it('describes a subset as "% less", never as a per-100 diff rate', async () => {
    const host = mount({
      pair: pair({ relation: 'subset', jaccard: 0.758, norm_chars_a: 1000, norm_chars_b: 750 }),
      bookA: book('book-a', { title: 'Full', char_count: 1000 }),
      bookB: book('book-b', { title: 'Trimmed', char_count: 750 })
    });
    await flush();

    const desc = host.querySelector('.similar-pair-desc')?.textContent ?? '';
    expect(desc).toContain('25%');
    expect(desc).not.toMatch(/per 100|in every 100/);
    // The trimmed copy carries the absolute character shortfall.
    expect(host.textContent).toContain('250 fewer');
  });

  it('describes an evenly-different pair with the Mash per-100 rate and no badge', async () => {
    const host = mount({
      pair: pair({ relation: 'near_identical', jaccard: 0.85 }),
      bookA: book('book-a', { char_count: 1000 }),
      bookB: book('book-b', { char_count: 1000 })
    });
    await flush();

    const desc = host.querySelector('.similar-pair-desc')?.textContent ?? '';
    // estimatedDiffPer100(0.85) === 2 under k = 5.
    expect(desc).toContain('2');
    expect(desc).toMatch(/in every 100/);
    expect(host.querySelector('.similar-badge')).toBeNull();
  });

  it('shows each book the source count handed down for it', async () => {
    const host = mount({
      pair: pair(),
      bookA: book('book-a'),
      bookB: book('book-b'),
      sourceCountA: 3,
      sourceCountB: 1
    });
    await flush();

    const rows = bookRows(host);
    const sourceValue = (row: HTMLElement) =>
      [...row.querySelectorAll('.similar-stat')]
        .find((stat) => stat.querySelector('dt')?.textContent === 'Sources')
        ?.querySelector('dd')?.textContent;
    expect(sourceValue(rows[0])).toBe('3');
    expect(sourceValue(rows[1])).toBe('1');
  });

  it('hides the delete action on a read-only shelf', async () => {
    const host = mount({
      pair: pair(),
      bookA: book('book-a'),
      bookB: book('book-b'),
      readOnly: true
    });
    await flush();

    expect(host.querySelector('.similar-delete')).toBeNull();
    // Open stays available so results are still inspectable read-only.
    expect(host.querySelector('.similar-open')).not.toBeNull();
  });

  it('confirming a delete warns when the fuller copy is the target, then deletes and emits', async () => {
    const onDeleted = vi.fn();
    const host = mount({
      pair: pair({ relation: 'subset', jaccard: 0.758, norm_chars_a: 1000, norm_chars_b: 750 }),
      bookA: book('book-a', { title: 'Full', char_count: 1000 }),
      bookB: book('book-b', { title: 'Trimmed', char_count: 750 }),
      onDeleted
    });
    await flush();

    // Top row is the fuller copy; deleting it must surface the comparison.
    host.querySelector<HTMLButtonElement>('.similar-book-row .similar-delete')!.click();
    await flush();

    const desc = host.querySelector('.dm-desc')?.textContent ?? '';
    expect(host.querySelector('.dm-item')?.textContent).toBe('Full');
    expect(desc).toContain('more complete');
    expect(desc).toContain('750'); // the other copy's character count
    expect(desc).toContain('Trimmed');

    host.querySelector<HTMLButtonElement>('.dm-confirm')!.click();
    await flush();

    expect(mocks.deleteBook).toHaveBeenCalledWith('book-a');
    expect(onDeleted).toHaveBeenCalledWith('book-a');
  });

  it('falls back to an id-titled stand-in when a book is missing from the listing', async () => {
    const host = mount({
      pair: pair({ a: 'ghost-a', b: 'ghost-b' }),
      bookA: undefined,
      bookB: undefined
    });
    await flush();

    expect(host.textContent).toContain('ghost-a');
    expect(host.textContent).toContain('ghost-b');
    // An unmatched book has no char_count, so the stat reads as unknown.
    expect(host.textContent).toContain('—');
  });
});
