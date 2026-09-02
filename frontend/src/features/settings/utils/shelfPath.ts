/**
 * Whether a typed shelf directory is absolute, matching what the desktop
 * backend accepts (`normalizeDesktopShelfDirectory` in `desktop/shelves.go`).
 *
 * The check runs in the renderer, which cannot see which OS it is driving, so
 * it accepts every shape Go's `filepath.IsAbs` accepts on any of the desktop
 * platforms: a POSIX path, a Windows drive path, and a UNC share. Being
 * permissive is the right way round — the form only refuses what the backend
 * would certainly refuse, and the backend still has the final say.
 */
export function isAbsoluteShelfPath(dir: string): boolean {
  const path = dir.trim();
  if (path === '') {
    return false;
  }
  if (path.startsWith('/')) {
    return true;
  }
  // `C:\books`, `C:/books`, and the UNC `\\server\share`.
  return /^[A-Za-z]:[\\/]/.test(path) || path.startsWith('\\\\');
}
