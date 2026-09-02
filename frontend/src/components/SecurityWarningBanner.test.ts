// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import SecurityWarningBanner from './SecurityWarningBanner.vue';
import { setLocale } from '@/i18n';

type InjectedSecurity = {
  token?: string;
  tokenHeader?: string;
  insecurePublicAccess?: boolean;
};

const mounted: Array<{ app: App; host: HTMLElement }> = [];

function setInjectedSecurity(value: InjectedSecurity | undefined): void {
  (window as unknown as { __PLAINSHELF_SECURITY__?: InjectedSecurity }).__PLAINSHELF_SECURITY__ =
    value;
}

function mountBanner(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(SecurityWarningBanner) }));
  app.mount(host);
  mounted.push({ app, host });
  return host;
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  while (mounted.length) {
    const { app, host } = mounted.pop()!;
    app.unmount();
    host.remove();
  }
  setInjectedSecurity(undefined);
});

describe('SecurityWarningBanner', () => {
  // The three security postures the ticket names, expressed as what the Go
  // server injects for each: only "none + non-loopback" carries the flag.
  it('shows the warning for mode none bound to a non-loopback address', () => {
    setInjectedSecurity({ insecurePublicAccess: true });
    const host = mountBanner();

    const panel = host.querySelector('.security-warning__panel');
    expect(panel).not.toBeNull();
    // The text names the consequence, not the config key.
    expect(host.textContent).toContain('read, change, and delete');
    // And links to the deployment guide.
    const link = host.querySelector<HTMLAnchorElement>('.security-warning__link');
    expect(link?.getAttribute('href')).toContain('docs/development/docker.md');
  });

  it('stays hidden for mode none bound to loopback (nothing injected)', () => {
    // A loopback none injects no bootstrap at all.
    setInjectedSecurity(undefined);
    const host = mountBanner();
    expect(host.querySelector('.security-warning')).toBeNull();
  });

  it('stays hidden for mode local_token regardless of address', () => {
    setInjectedSecurity({ token: 'abc', tokenHeader: 'X-PlainShelf-Token' });
    const host = mountBanner();
    expect(host.querySelector('.security-warning')).toBeNull();
  });

  it('collapses to a persistent badge and can be re-expanded, never fully dismissed', async () => {
    setInjectedSecurity({ insecurePublicAccess: true });
    const host = mountBanner();

    host.querySelector<HTMLButtonElement>('.security-warning__collapse')!.click();
    await nextTick();

    // Collapsed: the full panel is gone but a badge remains — never nothing.
    expect(host.querySelector('.security-warning__panel')).toBeNull();
    expect(host.querySelector('.security-warning__badge')).not.toBeNull();

    host.querySelector<HTMLButtonElement>('.security-warning__badge')!.click();
    await nextTick();

    expect(host.querySelector('.security-warning__panel')).not.toBeNull();
  });
});
