// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { guardFileDrops } from './wailsDropGuard';

const removals: (() => void)[] = [];

function listen(target: EventTarget, type: string, listener: (event: DragEvent) => void): void {
  target.addEventListener(type, listener as EventListener);
  removals.push(() => target.removeEventListener(type, listener as EventListener));
}

beforeAll(() => {
  guardFileDrops(document.documentElement);
});

afterEach(() => {
  for (const remove of removals.splice(0)) remove();
  document.body.replaceChildren();
});

function dragEvent(type: string, types: string[]): DragEvent {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(event, 'dataTransfer', { value: { types, dropEffect: 'copy' } });
  return event as DragEvent;
}

function zoneUnderRuntime(type: string): { zone: HTMLElement; runtime: ReturnType<typeof vi.fn> } {
  const zone = document.createElement('div');
  document.body.append(zone);
  const runtime = vi.fn((event: DragEvent) => {
    event.preventDefault();
    event.dataTransfer!.dropEffect = 'none';
  });
  listen(document.documentElement, type, runtime);
  return { zone, runtime };
}

describe('guardFileDrops', () => {
  it.each(['dragenter', 'dragover'])('keeps an accepted file %s from the runtime', (type) => {
    const { zone, runtime } = zoneUnderRuntime(type);
    listen(zone, type, (event) => event.preventDefault());

    const event = dragEvent(type, ['Files']);
    zone.dispatchEvent(event);

    expect(runtime).not.toHaveBeenCalled();
    expect(event.dataTransfer!.dropEffect, 'an accepted drop must stay droppable').toBe('copy');
  });

  it('lets an unaccepted file drag reach the runtime and document', () => {
    const { zone, runtime } = zoneUnderRuntime('dragover');
    const onDocument = vi.fn();
    listen(document, 'dragover', onDocument);

    zone.dispatchEvent(dragEvent('dragover', ['Files']));

    expect(runtime).toHaveBeenCalledOnce();
    expect(onDocument).toHaveBeenCalledOnce();
  });

  it('lets an accepted in-page drag reach the runtime', () => {
    const { zone, runtime } = zoneUnderRuntime('dragover');
    listen(zone, 'dragover', (event) => event.preventDefault());

    zone.dispatchEvent(dragEvent('dragover', ['text/plain']));

    expect(runtime).toHaveBeenCalledOnce();
  });
});
