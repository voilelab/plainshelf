import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'lab.voile.plainshelf',
  appName: 'PlainShelf',
  // Vite builds the SPA into frontend/dist; Capacitor packages that as the
  // native web assets. Run `npm run build` before `npx cap sync`.
  webDir: 'dist',
  server: {
    // Self-hosted PlainShelf servers are typically plain HTTP on the LAN, which
    // Android blocks by default (cleartext disabled since API 28). This also
    // covers CapacitorHttp's native requests.
    cleartext: true
  },
  plugins: {
    // Route fetch/XMLHttpRequest through native networking on device. Native
    // requests are not subject to WebView CORS, so the app can talk to a
    // PlainShelf server on another origin without adding the app's origin
    // (https://localhost) to the server's allowed_origins.
    CapacitorHttp: {
      enabled: true
    }
  }
};

export default config;
