import IconCheck from '~icons/tabler/check';
import IconChevronRight from '~icons/tabler/chevron-right';
import IconCloud from '~icons/tabler/cloud';
import IconDevices from '~icons/tabler/devices';
import IconFolder from '~icons/tabler/folder';
import IconFolderPlus from '~icons/tabler/folder-plus';
import IconQrcode from '~icons/tabler/qrcode';
import IconServer from '~icons/tabler/server';
import IconArrowLeft from '~icons/tabler/arrow-left';
import IconBaselineDensityMedium from '~icons/tabler/baseline-density-medium';
import IconBooks from '~icons/tabler/books';
import IconCopy from '~icons/tabler/copy';
import IconDownload from '~icons/tabler/download';
import IconFileText from '~icons/tabler/file-text';
import IconHistory from '~icons/tabler/history';
import IconLayoutDashboard from '~icons/tabler/layout-dashboard';
import IconLayoutGrid from '~icons/tabler/layout-grid';
import IconListDetails from '~icons/tabler/list-details';
import IconMenu2 from '~icons/tabler/menu-2';
import IconSettings from '~icons/tabler/settings';
import IconTrash from '~icons/tabler/trash';
import IconVersions from '~icons/tabler/versions';

export const ICONS = {
  dashboard: IconLayoutDashboard,
  library: IconBooks,
  'recently-read': IconHistory,
  'duplicate-content': IconCopy,
  'similar-content': IconVersions,
  trash: IconTrash,
  downloads: IconDownload,
  logs: IconFileText,
  settings: IconSettings,
  'view-list': IconListDetails,
  'view-card': IconLayoutGrid,
  'view-title': IconBaselineDensityMedium,
  menu: IconMenu2,
  back: IconArrowLeft,
  check: IconCheck,
  'chevron-right': IconChevronRight,
  cloud: IconCloud,
  devices: IconDevices,
  folder: IconFolder,
  'folder-plus': IconFolderPlus,
  qrcode: IconQrcode,
  server: IconServer
} as const;

export type IconName = keyof typeof ICONS;
