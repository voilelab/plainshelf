import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'lab.voile.plainshelf',
  appName: 'PlainShelf',
  // Vite builds the SPA into frontend/dist; Capacitor packages that as the
  // native web assets. Run `npm run build` before `npx cap sync`.
  webDir: 'dist'
};

export default config;
