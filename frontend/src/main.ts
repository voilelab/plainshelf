import { loadPreferences } from '@/storage/preferences';

async function start(): Promise<void> {
  await loadPreferences();
  const { bootstrap } = await import('./bootstrap');
  await bootstrap();
}

void start();
