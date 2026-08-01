import { describe, expect, it, vi } from 'vitest';
import { handleLibraryMobileBack, type MobileBackActions } from './mobileBack';

function actions(overrides: Partial<MobileBackActions> = {}): MobileBackActions {
  return {
    selectionActive: false,
    downloadRunning: false,
    clearSelection: vi.fn(),
    goBack: vi.fn(),
    exitApp: vi.fn(),
    ...overrides
  };
}

describe('handleLibraryMobileBack', () => {
  it('clears selection before navigating or exiting', () => {
    const handlers = actions({ selectionActive: true });
    handleLibraryMobileBack({ canGoBack: false }, handlers);
    expect(handlers.clearSelection).toHaveBeenCalledOnce();
    expect(handlers.goBack).not.toHaveBeenCalled();
    expect(handlers.exitApp).not.toHaveBeenCalled();
  });

  it('uses web history when Capacitor reports a previous entry', () => {
    const handlers = actions();
    handleLibraryMobileBack({ canGoBack: true }, handlers);
    expect(handlers.goBack).toHaveBeenCalledOnce();
    expect(handlers.exitApp).not.toHaveBeenCalled();
  });

  it('exits Android when there is no history entry', () => {
    const handlers = actions();
    handleLibraryMobileBack({ canGoBack: false }, handlers);
    expect(handlers.goBack).not.toHaveBeenCalled();
    expect(handlers.exitApp).toHaveBeenCalledOnce();
  });
});
