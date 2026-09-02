// @vitest-environment jsdom
import { createApp, h, ref, type App, type Ref, type VNode } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import ConfirmModal from './ConfirmModal.vue';
import { setLocale } from '@/i18n';

interface Harness {
  app: App;
  open: Ref<boolean>;
  cancels: number[];
  confirms: number[];
}

// `open` starts false and is flipped after mount: ConfirmModal focuses through a
// watcher on that transition, so a dialog mounted already-open never focuses
// anything and the assertions below would be vacuous.
function mount(props: Record<string, unknown> = {}, body?: () => VNode): Harness {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const open = ref(false);
  const cancels: number[] = [];
  const confirms: number[] = [];
  const app = createApp({
    render: () =>
      h(
        ConfirmModal,
        {
          open: open.value,
          title: 'Confirm delete',
          onCancel: () => cancels.push(1),
          onConfirm: () => confirms.push(1),
          ...props
        },
        { default: body ?? (() => h('p', 'Delete this?')) }
      )
  });
  app.mount(host);
  return { app, open, cancels, confirms };
}

// The dialog is portalled to document.body (reka DialogPortal), so nothing
// interesting lives inside the mount host.
function dialogRole(): string | null {
  const el = document.querySelector('[role="alertdialog"], [role="dialog"]');
  return el ? el.getAttribute('role') : null;
}

function buttonByText(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find(
    (el) => el.textContent?.trim() === text
  );
  if (!button) throw new Error(`no button with text "${text}"`);
  return button as HTMLButtonElement;
}

// Both `cancelable: true`s matter. Reka dismisses unless the layer's handler
// calls preventDefault(), and preventDefault() on a non-cancelable event is
// silently a no-op — so a plain `new KeyboardEvent('keydown', ...)` dismisses
// every dialog regardless of what the component decided.
function pressEscape(): void {
  document.dispatchEvent(
    new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
  );
}

// jsdom has no PointerEvent; reka's outside-pointer detection only reads
// `button`/`ctrlKey`, which MouseEvent carries.
function clickBackdrop(): void {
  document.body.dispatchEvent(
    new MouseEvent('pointerdown', { bubbles: true, cancelable: true })
  );
  document.body.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
}

// Reka focuses the cancel element from AlertDialogContent's open-auto-focus
// handler, a tick after the content mounts, so give the scheduler a beat.
async function settle(): Promise<void> {
  for (let i = 0; i < 8; i += 1) {
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

let mounted: App | null = null;

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  document.body.innerHTML = '';
});

async function openWith(
  props: Record<string, unknown> = {},
  body?: () => VNode
): Promise<Harness> {
  const harness = mount(props, body);
  mounted = harness.app;
  harness.open.value = true;
  await settle();
  return harness;
}

// A form dialog's slot: the field a caller would point `initialFocus` at.
function nameField(): VNode {
  return h('input', { id: 'shelf-name', type: 'text' });
}

describe('ConfirmModal danger variant', () => {
  it('announces itself as an alert dialog', async () => {
    await openWith({ variant: 'danger' });

    expect(dialogRole()).toBe('alertdialog');
  });

  it('opens with the cancel button focused, not the destructive one', async () => {
    await openWith({ variant: 'danger' });

    expect(document.activeElement).toBe(buttonByText('Cancel'));
  });

  it('does not close when the backdrop is clicked', async () => {
    const { cancels } = await openWith({ variant: 'danger' });

    clickBackdrop();
    await settle();

    expect(cancels).toHaveLength(0);
    expect(dialogRole()).toBe('alertdialog');
  });

  it('treats Escape as cancel', async () => {
    const { cancels } = await openWith({ variant: 'danger' });

    pressEscape();
    await settle();

    expect(cancels).toHaveLength(1);
  });

  it('ignores Escape while the operation is running', async () => {
    const { cancels } = await openWith({ variant: 'danger', busy: true });

    pressEscape();
    await settle();

    expect(cancels).toHaveLength(0);
  });

  it('emits cancel exactly once when the cancel button is clicked', async () => {
    const { cancels } = await openWith({ variant: 'danger' });

    buttonByText('Cancel').click();
    await settle();

    expect(cancels).toHaveLength(1);
  });

  it('leaves the dialog open after confirm so the parent can show progress', async () => {
    const { confirms, cancels } = await openWith({ variant: 'danger' });

    buttonByText('Confirm').click();
    await settle();

    expect(confirms).toHaveLength(1);
    expect(cancels).toHaveLength(0);
    expect(dialogRole()).toBe('alertdialog');
  });
});

describe('ConfirmModal default variant', () => {
  it('stays an ordinary dialog focused on confirm', async () => {
    await openWith({});

    expect(dialogRole()).toBe('dialog');
    expect(document.activeElement).toBe(buttonByText('Confirm'));
  });

  it('closes on a backdrop click', async () => {
    const { cancels } = await openWith({});

    clickBackdrop();
    await settle();

    expect(cancels.length).toBeGreaterThan(0);
  });

  it('still honours close-on-backdrop', async () => {
    const { cancels } = await openWith({ closeOnBackdrop: false });

    clickBackdrop();
    await settle();
    pressEscape();
    await settle();

    expect(cancels).toHaveLength(0);
  });
});

describe('ConfirmModal initial focus', () => {
  it('puts the caret in the field the caller names', async () => {
    await openWith({ initialFocus: '#shelf-name' }, nameField);

    expect(document.activeElement).toBe(document.getElementById('shelf-name'));
  });

  // The reason the prop exists: a form dialog's confirm button is disabled
  // until the form is valid, so the default path focuses nothing at all.
  it('reaches the field even when the confirm button is disabled', async () => {
    await openWith({ initialFocus: '#shelf-name', confirmDisabled: true }, nameField);

    expect(document.activeElement).toBe(document.getElementById('shelf-name'));
  });

  it('falls back to confirm when the selector matches nothing', async () => {
    await openWith({ initialFocus: '#not-here' }, nameField);

    expect(document.activeElement).toBe(buttonByText('Confirm'));
  });

  // The opt-in must not reach dialogs that did not ask for it, and must not
  // override the alert dialog's own focus contract.
  it('leaves an unasked dialog focused on confirm', async () => {
    await openWith({}, nameField);

    expect(document.activeElement).toBe(buttonByText('Confirm'));
  });

  it('still opens the danger variant on cancel even when a field is named', async () => {
    await openWith({ variant: 'danger', initialFocus: '#shelf-name' }, nameField);

    expect(document.activeElement).toBe(buttonByText('Cancel'));
  });
});
