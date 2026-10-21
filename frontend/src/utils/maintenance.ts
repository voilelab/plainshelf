import type { SidebarNavIconName } from '@/types/sidebarNavIcon';

type MaintenanceNavKey = 'duplicate-content' | 'similar-content';

type MaintenanceNavIcon = Extract<MaintenanceNavKey, SidebarNavIconName>;

interface MaintenanceNavItem {
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
];
