/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { readFileMock, writeFileMock, deleteFileMock } = vi.hoisted(() => ({
  readFileMock: vi.fn(),
  writeFileMock: vi.fn(),
  deleteFileMock: vi.fn()
}));

vi.mock('@capacitor/filesystem', () => ({
  Directory: { Data: 'DATA' },
  Encoding: { UTF8: 'utf8' },
  Filesystem: {
    readFile: readFileMock,
    writeFile: writeFileMock,
    deleteFile: deleteFileMock,
    mkdir: vi.fn(),
    rmdir: vi.fn()
  }
}));

const { FilesystemMobileCoverCache } = await import('./filesystemMobileCoverCache');

/** base64 of the given bytes. */
function b64(...bytes: number[]): string {
  return btoa(String.fromCharCode(...bytes));
}

/**
 * A Blob that can be read back.
 *
 * jsdom's Blob has no arrayBuffer(), which blobToBase64 needs; a real browser
 * and the Android WebView both do. Polyfilled per blob rather than globally so
 * the gap stays visible here instead of looking like production behaviour.
 */
function imageBlob(bytes: number[]): Blob {
  const blob = new Blob([new Uint8Array(bytes)]);
  Object.defineProperty(blob, 'arrayBuffer', {
    value: async () => new Uint8Array(bytes).buffer
  });
  return blob;
}

const JPEG = [0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0];
const PNG = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0];
const GIF = [0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0, 0, 0, 0, 0, 0];
const WEBP = [...'RIFF'].map((c) => c.charCodeAt(0))
  .concat([0, 0, 0, 0])
  .concat([...'WEBP'].map((c) => c.charCodeAt(0)));

describe('FilesystemMobileCoverCache', () => {
  beforeEach(() => {
    readFileMock.mockReset();
    writeFileMock.mockReset().mockResolvedValue(undefined);
    deleteFileMock.mockReset().mockResolvedValue(undefined);
  });

  it('writes the image itself, not a base64 document', async () => {
    await new FilesystemMobileCoverCache().setCover('book-1', imageBlob(JPEG));

    const [call] = writeFileMock.mock.calls;
    // No encoding: that is what makes the plugin decode the base64 to bytes.
    expect(call[0].encoding).toBeUndefined();
    expect(call[0].data).toBe(b64(...JPEG));
    // And no .json suffix on the path any more.
    expect(call[0].path).not.toMatch(/\.json$/);
  });

  it.each([
    ['image/jpeg', JPEG],
    ['image/png', PNG],
    ['image/gif', GIF],
    ['image/webp', WEBP]
  ])('recovers %s from the leading bytes rather than storing it', async (mime, bytes) => {
    readFileMock.mockResolvedValue({ data: b64(...bytes) });

    const blob = await new FilesystemMobileCoverCache().getCover('book-1');

    expect(blob?.type).toBe(mime);
  });

  // Without these, tightening the signatures would be untestable: the happy
  // cases pass either way, so only a near-miss can tell a full check from a
  // partial one.
  it.each([
    ['PNG missing its trailing CRLF/EOF bytes', [0x89, 0x50, 0x4e, 0x47, 0, 0, 0, 0, 0, 0, 0, 0]],
    ['GIF without a version', [0x47, 0x49, 0x46, 0, 0, 0, 0, 0, 0, 0, 0, 0]],
    ['RIFF that is not WebP', [...'RIFF'].map((c) => c.charCodeAt(0)).concat([0, 0, 0, 0], [...'WAVE'].map((c) => c.charCodeAt(0)))]
  ])('does not claim %s is that format', async (_label, bytes) => {
    readFileMock.mockResolvedValue({ data: b64(...bytes) });

    const blob = await new FilesystemMobileCoverCache().getCover('book-1');

    expect(blob?.type).toBe('');
  });

  it('still returns the image when the header is not one it knows', async () => {
    readFileMock.mockResolvedValue({ data: b64(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12) });

    const blob = await new FilesystemMobileCoverCache().getCover('book-1');

    // Untyped rather than absent: the decoder sniffs the bytes anyway, so a
    // cover the shelf accepted but this does not recognise still renders.
    expect(blob).not.toBeNull();
    expect(blob?.type).toBe('');
  });

  it('reports a missing cover as a miss', async () => {
    readFileMock.mockRejectedValue(new Error('File does not exist.'));

    expect(await new FilesystemMobileCoverCache().getCover('book-1')).toBeNull();
  });

  it('drops the pre-binary file when the cover is rewritten', async () => {
    await new FilesystemMobileCoverCache().setCover('book-1', imageBlob(PNG));

    // A cover is rebuildable, so the old layout is cleaned up per book rather
    // than by a migration pass.
    expect(deleteFileMock.mock.calls.some(([arg]) => arg.path.endsWith('.json'))).toBe(true);
  });

  it('removes both layouts when a cover is deleted', async () => {
    await new FilesystemMobileCoverCache().deleteCover('book-1');

    const paths = deleteFileMock.mock.calls.map(([arg]) => arg.path);
    expect(paths.some((path) => path.endsWith('.json'))).toBe(true);
    expect(paths.some((path) => !path.endsWith('.json'))).toBe(true);
  });
});
