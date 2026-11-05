import { callWails, wailsHost } from '@/api/wailsBindings';

interface OpenBookBinding {
  OpenBookPackage: () => Promise<string>;
}

export function hasOpenBookBinding(): boolean {
  return wailsHost() === 'ReaderApp';
}

export async function openBookPackage(): Promise<void> {
  await callWails<OpenBookBinding, 'OpenBookPackage'>('ReaderApp', 'OpenBookPackage');
}
