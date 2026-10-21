import { registerPlugin, type PluginListenerHandle } from '@capacitor/core';

export interface LanShareStatus {
  running: boolean;
  url: string;
  token: string;
  qr_code: string;
  error: string;
  shelf_ids: string[];
  endedReason: string;
  idleLimitMinutes: number;
}

interface LanSharePlugin {
  getStatus(): Promise<LanShareStatus>;
  start(options: { shelfIds: string[] }): Promise<LanShareStatus>;
  rotateToken(): Promise<LanShareStatus>;
  stop(): Promise<LanShareStatus>;
  addListener(event: 'lanShareChanged', listener: (status: LanShareStatus) => void): Promise<PluginListenerHandle>;
}

export const LanShare = registerPlugin<LanSharePlugin>('PlainShelfLanShare');
