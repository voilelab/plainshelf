import { callWails, hasWailsMethod } from '@/api/wailsBindings';

interface OpenBookBinding {
  OpenBookPackage: () => Promise<string>;
}

export function hasOpenBookBinding(): boolean {
  return hasWailsMethod('ReaderApp', 'OpenBookPackage');
}

export async function openBookPackage(): Promise<void> {
  await callWails<OpenBookBinding, 'OpenBookPackage'>('ReaderApp', 'OpenBookPackage');
}
