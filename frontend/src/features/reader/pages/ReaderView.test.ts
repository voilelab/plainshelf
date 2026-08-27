// @vitest-environment jsdom
import { createApp, defineComponent, h, ref, type App, type Ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Controllable reader state shared with the useReader mock. The keydown guard
// reads currentSectionIndex/sections directly, so tests drive them here and
// assert on the navigation spies.
const reader = vi.hoisted(() => {
  return {
    currentSectionIndex: undefined as unknown as Ref<number>,
    sections: undefined as unknown as Ref<unknown[]>,
    goPrevSection: vi.fn(),
    goNextSection: vi.fn()
  };
});

vi.mock('@/features/reader/composables/useReader', async () => {
  const { ref: makeRef } = await vi.importActual<typeof import('vue')>('vue');
  reader.currentSectionIndex = makeRef(1);
  reader.sections = makeRef([{}, {}, {}]);
  return {
    useReader: () => ({
      title: makeRef(''),
      bookFormat: makeRef('txt'),
      currentSourceId: makeRef(''),
      sections: reader.sections,
      currentSectionIndex: reader.currentSectionIndex,
      currentSection: makeRef(null),
      progress: makeRef(null),
      loading: makeRef(false),
      error: makeRef(null),
      saveError: makeRef(null),
      readerRef: makeRef(null),
      fetchReaderData: vi.fn().mockResolvedValue(undefined),
      onScroll: vi.fn(),
      goPrevSection: reader.goPrevSection,
      goNextSection: reader.goNextSection,
      goToSection: vi.fn().mockResolvedValue(undefined),
      syncCurrentScroll: vi.fn().mockResolvedValue(undefined),
      flushReadingProgress: vi.fn().mockResolvedValue(undefined),
      startProgressAutosave: vi.fn(),
      stopProgressAutosave: vi.fn().mockResolvedValue(undefined)
    })
  };
});

vi.mock('@/features/reader/composables/useReaderPresentation', () => ({
  useMobileReaderPresentation: () => ref(false)
}));

vi.mock('@/features/reader/composables/useReaderSettings', async () => {
  const actual = await vi.importActual<
    typeof import('@/features/reader/composables/useReaderSettings')
  >('@/features/reader/composables/useReaderSettings');
  return {
    ...actual,
    useReaderSettings: () => ({
      fontSize: ref(20),
      fontFamily: ref('system'),
      isAtMinFontSize: ref(false),
      isAtMaxFontSize: ref(false),
      increaseFontSize: vi.fn(),
      decreaseFontSize: vi.fn(),
      setFontFamily: vi.fn()
    })
  };
});

vi.mock('@/features/reader/composables/useReadingHeartbeat', () => ({
  useReadingHeartbeat: () => ({ start: vi.fn(), stop: vi.fn() })
}));

vi.mock('@/composables/useDocumentTitle', () => ({
  useDocumentTitle: vi.fn()
}));

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'book-1' }, query: {} }),
  useRouter: () => ({}),
  onBeforeRouteLeave: vi.fn()
}));

// The desktop reader owns the real <button> controls the bug is about, so the
// stub renders focusable buttons and forwards the modal event. Focusing one of
// these reproduces "focus parked on a button" without the full reader tree.
// Hoisted so the stub factories below (also hoisted) can read it.
const READER_PROPS = vi.hoisted(() => [
  'style',
  'bookId',
  'title',
  'bookFormat',
  'sourceId',
  'sections',
  'currentSectionIndex',
  'currentSection',
  'progressPercent',
  'loading',
  'error',
  'saveError',
  'isAtMinFontSize',
  'isAtMaxFontSize'
]);

vi.mock('@/features/reader/components/DesktopReaderView.vue', () => ({
  default: defineComponent({
    props: READER_PROPS,
    emits: ['open-chapter-modal', 'open-font-modal', 'previous-section', 'next-section'],
    setup(_, { emit }) {
      return () =>
        h('div', { class: 'desktop-reader' }, [
          h('button', { class: 'nav-prev', onClick: () => emit('previous-section') }, 'prev'),
          h('button', { class: 'nav-next', onClick: () => emit('next-section') }, 'next'),
          h('button', { class: 'open-chapter', onClick: () => emit('open-chapter-modal') }, 'chapters')
        ]);
    }
  })
}));

vi.mock('@/features/reader/components/MobileReaderView.vue', () => ({
  default: defineComponent({ props: READER_PROPS, setup: () => () => h('div') })
}));

vi.mock('@/features/reader/components/ChapterModal.vue', () => ({
  default: defineComponent({
    props: ['open', 'sections', 'currentSectionIndex'],
    emits: ['close', 'select-section'],
    setup: () => () => h('div')
  })
}));

vi.mock('@/features/reader/components/FontSelectionModal.vue', () => ({
  default: defineComponent({
    props: ['open', 'fontFamily'],
    emits: ['close', 'select'],
    setup: () => () => h('div')
  })
}));

import ReaderView from './ReaderView.vue';

let mounted: { app: App; host: HTMLElement } | null = null;

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(ReaderView) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function press(key: 'ArrowLeft' | 'ArrowRight'): void {
  document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
}

beforeEach(() => {
  reader.goPrevSection.mockReset();
  reader.goNextSection.mockReset();
  reader.currentSectionIndex.value = 1;
  reader.sections.value = [{}, {}, {}];
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  document.body.innerHTML = '';
});

describe('ReaderView keyboard chapter navigation', () => {
  it('changes chapter with arrow keys when nothing is focused', async () => {
    mount();
    await flush();

    press('ArrowLeft');
    expect(reader.goPrevSection).toHaveBeenCalledTimes(1);

    press('ArrowRight');
    expect(reader.goNextSection).toHaveBeenCalledTimes(1);
  });

  it('still changes chapter when focus rests on a reader button', async () => {
    const host = mount();
    await flush();

    // Reproduce the reported bug: focus parked on a chapter/side-action button.
    host.querySelector<HTMLButtonElement>('.nav-next')!.focus();
    expect(document.activeElement).toBeInstanceOf(HTMLButtonElement);

    press('ArrowRight');
    expect(reader.goNextSection).toHaveBeenCalledTimes(1);

    press('ArrowLeft');
    expect(reader.goPrevSection).toHaveBeenCalledTimes(1);
  });

  it('leaves the arrow keys to move the caret inside text inputs', async () => {
    mount();
    await flush();

    const input = document.createElement('input');
    document.body.append(input);
    input.focus();
    press('ArrowLeft');
    press('ArrowRight');
    expect(reader.goPrevSection).not.toHaveBeenCalled();
    expect(reader.goNextSection).not.toHaveBeenCalled();
    input.remove();

    const textarea = document.createElement('textarea');
    document.body.append(textarea);
    textarea.focus();
    press('ArrowRight');
    expect(reader.goNextSection).not.toHaveBeenCalled();
    textarea.remove();

    const editable = document.createElement('div');
    editable.setAttribute('contenteditable', 'true');
    editable.tabIndex = 0;
    document.body.append(editable);
    editable.focus();
    press('ArrowRight');
    expect(reader.goNextSection).not.toHaveBeenCalled();
    editable.remove();
  });

  it('does not step past the first or last chapter', async () => {
    mount();
    await flush();

    reader.currentSectionIndex.value = 0;
    press('ArrowLeft');
    expect(reader.goPrevSection).not.toHaveBeenCalled();

    reader.currentSectionIndex.value = reader.sections.value.length - 1;
    press('ArrowRight');
    expect(reader.goNextSection).not.toHaveBeenCalled();
  });

  it('ignores the arrow keys while a modal is open', async () => {
    const host = mount();
    await flush();

    host.querySelector<HTMLButtonElement>('.open-chapter')!.click();
    await flush();

    press('ArrowLeft');
    press('ArrowRight');
    expect(reader.goPrevSection).not.toHaveBeenCalled();
    expect(reader.goNextSection).not.toHaveBeenCalled();
  });
});
