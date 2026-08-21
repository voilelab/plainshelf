import { getServerMode } from '@/api/mode';
import { registerShell } from '@/providers/shell';
import { ServerBookshelfProvider } from '@/providers/serverBookshelfProvider';
import { ReaderBookshelfProvider } from './readerBookshelfProvider';
import { installReaderRouterGuards } from './routerGuard';

/**
 * Brings up the reader shell when the backend turns out to be one.
 *
 * The standalone reading binary (cmd/plainshelf-read) serves this same bundle,
 * so the client cannot tell what it is talking to from the runtime alone — it
 * asks, and `GET /api/mode` answers `reader`. Returns whether the shell was
 * installed, so a caller can tell a reading server from an ordinary one.
 *
 * Called during bootstrap, before anything reaches for a provider: registering a
 * shell after that would be too late, because the provider is built once on
 * first use and kept.
 *
 * A failed probe leaves the app as a plain web client. That is the safe way
 * round — a shell that is not installed costs a reader some blocked routes it
 * would have liked, while one installed by mistake would take the editing UI
 * away from a full server.
 */
export async function installReaderShell(): Promise<boolean> {
  let mode;
  try {
    mode = await getServerMode();
  } catch {
    return false;
  }

  if (mode.mode !== 'reader') {
    return false;
  }

  registerShell({
    createProvider: () => new ReaderBookshelfProvider(new ServerBookshelfProvider()),
    installRouterGuards: installReaderRouterGuards
  });

  return true;
}
