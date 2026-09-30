export function hasSupportedExtension(filename: string, pattern: RegExp): boolean {
  return pattern.test(filename);
}

export function readSelectedFiles(event: Event): File[] {
  const target = event.target as HTMLInputElement;
  return Array.from(target.files ?? []);
}

export function hasFileTransfer(dataTransfer: DataTransfer | null | undefined): boolean {
  return Array.from(dataTransfer?.types ?? []).includes('Files');
}

export function readDroppedFiles(event: DragEvent): File[] {
  return Array.from(event.dataTransfer?.files ?? []);
}

// Handles both POSIX and Windows separators, so a desktop local path shows its
// filename rather than the whole path in the import list.
export function basenameFromPath(path: string): string {
  const segments = path.split(/[/\\]/);
  return segments[segments.length - 1] || path;
}

export function deriveTitleFromFilename(filename: string): string {
  const normalizedFilename = filename.trim();
  if (normalizedFilename.length === 0) {
    return filename;
  }

  const dotIndex = normalizedFilename.lastIndexOf('.');
  const withoutExt = dotIndex > 0 ? normalizedFilename.slice(0, dotIndex) : normalizedFilename;
  const title = withoutExt.trim();
  return title.length > 0 ? title : normalizedFilename;
}
