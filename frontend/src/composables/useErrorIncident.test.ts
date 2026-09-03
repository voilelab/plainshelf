import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { reportIncident, reportUnhandledError, useErrorIncident } from './useErrorIncident';

const { incident, dismissIncident } = useErrorIncident();

beforeEach(() => {
  dismissIncident();
});

describe('useErrorIncident', () => {
  it('starts with nothing to show', () => {
    expect(incident.value).toBe('');
  });

  it('keeps only the most recent reference', () => {
    reportIncident('ABCD2345');
    reportIncident('c-K7MQ4XZB');
    expect(incident.value).toBe('c-K7MQ4XZB');
  });

  it('ignores a blank reference', () => {
    reportIncident('ABCD2345');
    reportIncident('   ');
    expect(incident.value).toBe('ABCD2345');
  });

  it('clears on dismissal', () => {
    reportIncident('ABCD2345');
    dismissIncident();
    expect(incident.value).toBe('');
  });
});

describe('reportUnhandledError', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('logs and shows the same client-side reference', () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {});
    const err = new Error('render blew up');

    reportUnhandledError(err, 'render function');

    expect(incident.value).toMatch(/^c-[23456789ABCDEFGHJKMNPQRSTVWXYZ]{8}$/);
    expect(logged).toHaveBeenCalledWith(
      `Unhandled Vue error (render function) [${incident.value}]`,
      err
    );
  });
});
