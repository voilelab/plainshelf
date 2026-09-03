import { describe, expect, it, vi } from 'vitest';
import { newClientIncidentID } from './incident';

// The same shape internal/logutil/requestid.go produces, plus the prefix.
const CLIENT_INCIDENT = /^c-[23456789ABCDEFGHJKMNPQRSTVWXYZ]{8}$/;

describe('newClientIncidentID', () => {
  it('mints a prefixed ID in the server ID alphabet', () => {
    for (let i = 0; i < 50; i += 1) {
      expect(newClientIncidentID()).toMatch(CLIENT_INCIDENT);
    }
  });

  it('does not repeat itself', () => {
    const ids = new Set(Array.from({ length: 200 }, () => newClientIncidentID()));
    expect(ids.size).toBe(200);
  });

  it('still mints an ID without Web Crypto', () => {
    const crypto = globalThis.crypto;
    vi.stubGlobal('crypto', undefined);
    try {
      expect(newClientIncidentID()).toMatch(CLIENT_INCIDENT);
    } finally {
      vi.stubGlobal('crypto', crypto);
    }
  });
});
