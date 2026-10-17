import { registerPlugin, type PluginListenerHandle } from '@capacitor/core';

/**
 * The Android app's LAN share: a read-only WebDAV export of ticked local
 * shelves, held open by a foreground service (LanShareService). Go binds the
 * listener only while it runs.
 */
export interface LanShareStatus {
  running: boolean;
  /** `http://<ip>:<port>/dav/`; empty while stopped. */
  url: string;
  token: string;
  shelf_ids: string[];
  /** Why the last share stopped on its own (idle, network); empty otherwise. */
  endedReason: string;
  idleLimitMinutes: number;
}

interface LanSharePlugin {
  getStatus(): Promise<LanShareStatus>;
  /** Rejects with "not connected to Wi-Fi" off a LAN. */
  start(options: { shelfIds: string[] }): Promise<LanShareStatus>;
  stop(): Promise<LanShareStatus>;
  addListener(event: 'lanShareChanged', listener: (status: LanShareStatus) => void): Promise<PluginListenerHandle>;
}

// No web implementation: only the native app has local shelves to share.
export const LanShare = registerPlugin<LanSharePlugin>('PlainShelfLanShare');
