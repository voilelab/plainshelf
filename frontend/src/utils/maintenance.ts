import type { SidebarNavIconName } from '@/types/sidebarNavIcon';

export type MaintenanceNavKey = 'duplicate-content' | 'similar-content' | 'incomplete';

export type MaintenanceNavIcon = Extract<MaintenanceNavKey, SidebarNavIconName>;

export interface MaintenanceNavItem {
  key: MaintenanceNavKey;
  labelKey: string;
  to: string;
  icon?: MaintenanceNavIcon;
}

export const MAINTENANCE_NAV_ITEMS: MaintenanceNavItem[] = [
  {
    key: 'duplicate-content',
    labelKey: 'maintenance.duplicateContent',
    to: '/duplicates',
    icon: 'duplicate-content'
  },
  {
    key: 'similar-content',
    labelKey: 'maintenance.similarContent',
    to: '/similar',
    icon: 'similar-content'
  },
  // The three "missing X" pages collapsed into one book-list filter: the library
  // renders `incomplete=1` (missing author, cover, or language) as its own view,
  // and the individual conditions are reachable by URL and the filter panel.
  {
    key: 'incomplete',
    labelKey: 'maintenance.incomplete',
    to: '/books?incomplete=1',
    icon: 'incomplete'
  }
];
