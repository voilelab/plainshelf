export function isAbsoluteShelfPath(dir: string): boolean {
  const path = dir.trim();
  if (path === '') {
    return false;
  }
  if (path.startsWith('/')) {
    return true;
  }
  return /^[A-Za-z]:[\\/]/.test(path) || path.startsWith('\\\\');
}
