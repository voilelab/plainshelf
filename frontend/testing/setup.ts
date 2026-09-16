/**
 * Runs before every test file. Its whole job is the teardown that used to be
 * each file's to remember — see testing/mount.ts.
 */
import { afterEach } from 'vitest';
import { unmountAll } from './mount';

afterEach(() => {
  unmountAll();
});
