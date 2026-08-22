import type { SidebarNavIconName } from '@/types/sidebarNavIcon';

export type MaintenanceNavKey = 'duplicate-content' | 'similar-content';

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
  }
  // The former "missing author/cover/language" entries are gone: each is now a
  // book-list filter reached by URL (?author=none etc.) and, later, the filter
  // panel — not a maintenance sidebar entry.
];
