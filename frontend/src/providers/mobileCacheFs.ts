import { Directory, Encoding, Filesystem, type WriteFileOptions } from '@capacitor/filesystem';

export const BASE_DIR = 'plainshelf-cache';
const SCOPES_DIR = `${BASE_DIR}/scopes`;
export const CACHE_DIRECTORY = Directory.Data;

export const UNSCOPED_DIR_NAME = '_unscoped';

export function encode(id: string): string {
  return encodeURIComponent(id);
}

export function hashComponent(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let low = 0x811c9dc5;
  let high = 0x01000193;

  for (const byte of bytes) {
    low = Math.imul(low ^ byte, 0x01000193) >>> 0;
    high = Math.imul(high ^ byte, 0x85ebca6b) >>> 0;
  }

  return low.toString(16).padStart(8, '0') + high.toString(16).padStart(8, '0');
}

export function scopeDir(scopeKey: string): string {
  return `${SCOPES_DIR}/${scopeKey ? encode(scopeKey) : UNSCOPED_DIR_NAME}`;
}

export async function removeCacheScope(scopeKey: string): Promise<void> {
  if (!scopeKey) {
    return;
  }
  await rmdirIgnoringMissing(scopeDir(scopeKey));
}

export async function readTextFile(path: string): Promise<string | null> {
  try {
    const result = await Filesystem.readFile({
      path,
      directory: CACHE_DIRECTORY,
      encoding: Encoding.UTF8
    });
    return typeof result.data === 'string' ? result.data : null;
  } catch {
    return null;
  }
}

export async function readJsonFile<T>(path: string): Promise<T | null> {
  const text = await readTextFile(path);
  if (text === null) {
    return null;
  }
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

// Capacitor Filesystem 8.1.3 web writeFile separately checks and creates the parent; concurrent mkdir throws "Directory already exists" (dist/esm/web.js).
async function writeFileCreatingParents(options: WriteFileOptions): Promise<void> {
  try {
    await Filesystem.writeFile(options);
    return;
  } catch (error) {
    if (!isDirectoryExistsError(error)) {
      throw error;
    }
  }

  await Filesystem.writeFile(options);
}

export async function writeTextFile(path: string, data: string): Promise<void> {
  await writeFileCreatingParents({
    path,
    data,
    directory: CACHE_DIRECTORY,
    encoding: Encoding.UTF8,
    recursive: true
  });
}

export async function writeJsonFile(path: string, value: unknown): Promise<void> {
  await writeTextFile(path, JSON.stringify(value));
}

// Capacitor Filesystem 8.1.3 readFile returns binary data as base64 when encoding is omitted (definitions.d.ts).
export async function readBinaryFile(path: string): Promise<string | null> {
  try {
    const result = await Filesystem.readFile({ path, directory: CACHE_DIRECTORY });
    return typeof result.data === 'string' ? result.data : null;
  } catch {
    return null;
  }
}

// Capacitor Filesystem 8.1.3 writeFile accepts base64 binary input when encoding is omitted (definitions.d.ts).
export async function writeBinaryFile(path: string, base64: string): Promise<void> {
  await writeFileCreatingParents({
    path,
    data: base64,
    directory: CACHE_DIRECTORY,
    recursive: true
  });
}

export async function deleteFileIgnoringMissing(path: string): Promise<void> {
  try {
    await Filesystem.deleteFile({ path, directory: CACHE_DIRECTORY });
  } catch (error) {
    if (!isMissingError(error)) {
      throw error;
    }
  }
}

export async function rmdirIgnoringMissing(path: string): Promise<void> {
  try {
    await Filesystem.rmdir({ path, directory: CACHE_DIRECTORY, recursive: true });
  } catch (error) {
    if (!isMissingError(error)) {
      throw error;
    }
  }
}

export function isMissingError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return /not exist|does not exist|enoent|no such file|not found/i.test(message);
}

function isDirectoryExistsError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return /already exist/i.test(message);
}

export async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = '';
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

export function base64ToBlob(base64: string, mime: string): Blob {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], mime ? { type: mime } : undefined);
}
