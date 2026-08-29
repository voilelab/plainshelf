/**
 * Every UI icon the app renders, resolved from Tabler Icons (MIT) at build time
 * by unplugin-icons. `~icons/tabler/*` compiles to a plain inline SVG component,
 * so nothing from Iconify reaches the bundle at runtime.
 *
 * Adding an icon means adding an import and a `ICONS` entry here; call sites
 * only ever name a key, which keeps swapping the icon set to an import rewrite.
 */
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
  menu: IconMenu2
} as const;

export type IconName = keyof typeof ICONS;
