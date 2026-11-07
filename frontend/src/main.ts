import { startWailsRuntime } from '@/api/wailsBindings';
import { loadPreferences } from '@/storage/preferences';

async function start(): Promise<void> {
  startWailsRuntime();
  await loadPreferences();
  const { bootstrap } = await import('./bootstrap');
  await bootstrap();
}

void start();
