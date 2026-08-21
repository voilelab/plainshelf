/**
 * The reader app's one native affordance, reached from the holding screen.
 *
 * Bound by the Wails app rather than served over HTTP: picking a folder is a
 * platform dialog, and the app reloads the window itself once a book is open,
 * so there is nothing to return to the caller.
 */
interface ReaderBindingWindow extends Window {
  go?: {
    main?: {
      ReaderApp?: {
        OpenBookPackage?: () => Promise<string>;
      };
    };
  };
}

export function hasOpenBookBinding(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }
  return Boolean((window as ReaderBindingWindow).go?.main?.ReaderApp?.OpenBookPackage);
}

export async function openBookPackage(): Promise<void> {
  const open = (window as ReaderBindingWindow).go?.main?.ReaderApp?.OpenBookPackage;
  if (!open) {
    throw new Error('OpenBookPackage binding not available');
  }

  await open();
}
