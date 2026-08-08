import { describe, expect, it, vi } from 'vitest';

import { buildAuthorizeUrl, generateRequestId, hostForLocationId, pollForToken } from './auth';
import { PCloudError } from './errors';

const CLIENT_ID = 'client-abc';
const REQUEST_ID = 'request-xyz';

function tokenResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  });
}

function pollOptions(fetchImpl: typeof fetch, overrides: Record<string, unknown> = {}) {
  return {
    clientId: CLIENT_ID,
    requestId: REQUEST_ID,
    fetchImpl,
    sleepImpl: async () => {},
    timeoutMs: 10_000,
    ...overrides
  };
}

describe('generateRequestId', () => {
  it('produces a 40-character id that differs between calls', () => {
    const a = generateRequestId();
    const b = generateRequestId();

    expect(a).toHaveLength(40);
    expect(a).toMatch(/^[A-Za-z0-9]+$/);
    expect(a).not.toBe(b);
  });
});

describe('buildAuthorizeUrl', () => {
  it('requests the poll_token flow and sends no redirect_uri', () => {
    const url = new URL(buildAuthorizeUrl(CLIENT_ID, REQUEST_ID));

    expect(url.origin + url.pathname).toBe('https://my.pcloud.com/oauth2/authorize');
    expect(url.searchParams.get('response_type')).toBe('poll_token');
    expect(url.searchParams.get('client_id')).toBe(CLIENT_ID);
    expect(url.searchParams.get('request_id')).toBe(REQUEST_ID);
    expect(url.searchParams.has('redirect_uri')).toBe(false);
  });

  it('refuses to build a URL without a client id or request id', () => {
    expect(() => buildAuthorizeUrl('  ', REQUEST_ID)).toThrow(PCloudError);
    expect(() => buildAuthorizeUrl(CLIENT_ID, ' ')).toThrow(PCloudError);
  });
});

describe('hostForLocationId', () => {
  it('maps location 2 to the EU host and everything else to the default', () => {
    expect(hostForLocationId(2)).toBe('eapi.pcloud.com');
    expect(hostForLocationId(1)).toBe('api.pcloud.com');
    expect(hostForLocationId(undefined)).toBe('api.pcloud.com');
  });
});

describe('pollForToken', () => {
  it('adopts the host that answered as the session host', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('eapi.pcloud.com')) {
        return Promise.resolve(tokenResponse({ result: 0, access_token: 'eu-token', locationid: 2 }));
      }
      return Promise.resolve(tokenResponse({ result: 2000, error: 'Log in failed.' }));
    });

    const session = await pollForToken(pollOptions(fetchImpl as unknown as typeof fetch));

    expect(session).toEqual({ accessToken: 'eu-token', host: 'eapi.pcloud.com' });
  });

  it('uses locationid even when the other regional polling host delivered the token', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('eapi.pcloud.com')) {
        return Promise.resolve(tokenResponse({ result: 0, access_token: 'us-token', locationid: 1 }));
      }
      return Promise.resolve(tokenResponse({ result: 2000, error: 'Not yet.' }));
    });

    const session = await pollForToken(pollOptions(fetchImpl as unknown as typeof fetch));

    expect(session).toEqual({ accessToken: 'us-token', host: 'api.pcloud.com' });
  });

  it('falls back to the answering host when an old response omits locationid', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('eapi.pcloud.com')) {
        return Promise.resolve(tokenResponse({ result: 0, access_token: 'legacy-token' }));
      }
      return Promise.resolve(tokenResponse({ result: 2000, error: 'Not yet.' }));
    });

    const session = await pollForToken(pollOptions(fetchImpl as unknown as typeof fetch));

    expect(session).toEqual({ accessToken: 'legacy-token', host: 'eapi.pcloud.com' });
  });

  it('keeps polling until the user approves', async () => {
    let calls = 0;
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('eapi.pcloud.com')) {
        return Promise.resolve(tokenResponse({ result: 2000, error: 'Log in failed.' }));
      }
      calls += 1;
      if (calls < 3) {
        return Promise.resolve(tokenResponse({ result: 2000, error: 'Not yet.' }));
      }
      return Promise.resolve(tokenResponse({ result: 0, access_token: 'us-token', locationid: 1 }));
    });

    const session = await pollForToken(pollOptions(fetchImpl as unknown as typeof fetch));

    expect(session).toEqual({ accessToken: 'us-token', host: 'api.pcloud.com' });
    expect(calls).toBe(3);
  });

  it('treats a transport failure as "not yet" rather than an abort', async () => {
    let calls = 0;
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('eapi.pcloud.com')) {
        return Promise.reject(new Error('network down'));
      }
      calls += 1;
      if (calls === 1) {
        return Promise.reject(new Error('network blip'));
      }
      return Promise.resolve(tokenResponse({ result: 0, access_token: 'us-token' }));
    });

    await expect(pollForToken(pollOptions(fetchImpl as unknown as typeof fetch))).resolves.toMatchObject({
      accessToken: 'us-token'
    });
  });

  it('rejects once the deadline passes with no approval', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(tokenResponse({ result: 2000, error: 'Not yet.' }));
    let now = 0;

    await expect(
      pollForToken(
        pollOptions(fetchImpl as unknown as typeof fetch, {
          timeoutMs: 100,
          nowImpl: () => {
            now += 60;
            return now;
          }
        })
      )
    ).rejects.toBeInstanceOf(PCloudError);
  });

  it('rejects without polling when the signal was already aborted', async () => {
    const controller = new AbortController();
    controller.abort();
    const fetchImpl = vi.fn();

    await expect(
      pollForToken(pollOptions(fetchImpl as unknown as typeof fetch, { signal: controller.signal }))
    ).rejects.toThrow(/cancelled/i);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('stops when the caller aborts', async () => {
    const controller = new AbortController();
    const fetchImpl = vi.fn().mockImplementation(() => {
      controller.abort();
      return Promise.resolve(tokenResponse({ result: 2000, error: 'Not yet.' }));
    });

    await expect(
      pollForToken(pollOptions(fetchImpl as unknown as typeof fetch, { signal: controller.signal }))
    ).rejects.toThrow(/cancelled/i);
  });
});
