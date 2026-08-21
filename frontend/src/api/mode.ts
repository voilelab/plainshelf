import { fetchJson, isMockApiMode } from './client';

/**
 * Which HTTP surface the backend mounts, as `GET /api/mode` reports it.
 *
 * `full` is an ordinary PlainShelf server or the desktop app: the whole API,
 * writable unless it says otherwise. `reader` is the standalone reading binary
 * (cmd/plainshelf-read), which serves the reading routes and nothing else — the
 * trash, log, setting and task routes are not merely refused there, they do not
 * exist, so the pages that call them have to be kept out of reach.
 *
 * Deliberately distinct from `readOnly`. A read-only server still serves the
 * whole surface and still administers itself; the two answer different
 * questions and a client needs both.
 */
export type ServerMode = 'full' | 'reader';

export interface ServerModeInfo {
  readOnly: boolean;
  mode: ServerMode;
}

interface ModeResponse {
  read_only?: unknown;
  mode?: unknown;
}

const DEFAULT_MODE: ServerModeInfo = { readOnly: false, mode: 'full' };

function toBoolean(value: unknown): boolean {
  return value === true || value === 'true' || value === 1 || value === '1';
}

// An unknown name is read as `full` rather than refused: a build that meets a
// mode it does not know about should offer the surface it has always offered,
// and let the routes themselves answer for what is missing.
function toServerMode(value: unknown): ServerMode {
  return value === 'reader' ? 'reader' : 'full';
}

/**
 * The one in-flight or resolved answer, shared by every caller.
 *
 * A server's mode is fixed for as long as the process it belongs to is running,
 * and two callers ask during startup — the reader shell, before the router's
 * first navigation, and MainLayout when it mounts — so one request serves both.
 * A failed request is not remembered, so a client that started offline can
 * still learn the mode later.
 */
let pending: Promise<ServerModeInfo> | null = null;

export async function getServerMode(): Promise<ServerModeInfo> {
  if (isMockApiMode()) {
    return DEFAULT_MODE;
  }

  if (!pending) {
    pending = fetchJson<ModeResponse>('/api/mode')
      .then((res) => ({
        readOnly: toBoolean(res?.read_only),
        mode: toServerMode(res?.mode)
      }))
      .catch((err) => {
        pending = null;
        throw err;
      });
  }

  return pending;
}

export async function getReadOnlyMode(): Promise<boolean> {
  return (await getServerMode()).readOnly;
}

/** Test seam: drops the cached answer so the next call issues a request. */
export function resetServerModeCache(): void {
  pending = null;
}
