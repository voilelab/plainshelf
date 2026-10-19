/**
 * The folder icons a user may pick, keyed by the name folder.json stores. The
 * server owns the list (shelf.FolderIcons); internal/repocheck fails when the
 * two differ, so a name is added in both places or neither.
 *
 * Imported one by one, like registry.ts, so only these icons reach the bundle.
 */
import type { Component } from 'vue';
import IconAlien from '~icons/tabler/alien';
import IconAtom from '~icons/tabler/atom';
import IconBolt from '~icons/tabler/bolt';
import IconBook from '~icons/tabler/book';
import IconBookmark from '~icons/tabler/bookmark';
import IconBooks from '~icons/tabler/books';
import IconBriefcase from '~icons/tabler/briefcase';
import IconCamera from '~icons/tabler/camera';
import IconCode from '~icons/tabler/code';
import IconCoffee from '~icons/tabler/coffee';
import IconCompass from '~icons/tabler/compass';
import IconCrown from '~icons/tabler/crown';
import IconDeviceGamepad2 from '~icons/tabler/device-gamepad-2';
import IconDiamond from '~icons/tabler/diamond';
import IconFlame from '~icons/tabler/flame';
import IconFlask from '~icons/tabler/flask';
import IconFlower from '~icons/tabler/flower';
import IconFolder from '~icons/tabler/folder';
import IconGhost from '~icons/tabler/ghost';
import IconHeart from '~icons/tabler/heart';
import IconLeaf from '~icons/tabler/leaf';
import IconMap from '~icons/tabler/map';
import IconMoodSmile from '~icons/tabler/mood-smile';
import IconMoon from '~icons/tabler/moon';
import IconMovie from '~icons/tabler/movie';
import IconMusic from '~icons/tabler/music';
import IconNotebook from '~icons/tabler/notebook';
import IconPalette from '~icons/tabler/palette';
import IconPaw from '~icons/tabler/paw';
import IconPlanet from '~icons/tabler/planet';
import IconPuzzle from '~icons/tabler/puzzle';
import IconRobot from '~icons/tabler/robot';
import IconRocket from '~icons/tabler/rocket';
import IconSchool from '~icons/tabler/school';
import IconShield from '~icons/tabler/shield';
import IconSkull from '~icons/tabler/skull';
import IconStar from '~icons/tabler/star';
import IconSun from '~icons/tabler/sun';
import IconSword from '~icons/tabler/sword';
import IconWorld from '~icons/tabler/world';

const FOLDER_ICONS: Readonly<Record<string, Component>> = {
  alien: IconAlien,
  atom: IconAtom,
  bolt: IconBolt,
  book: IconBook,
  bookmark: IconBookmark,
  books: IconBooks,
  briefcase: IconBriefcase,
  camera: IconCamera,
  code: IconCode,
  coffee: IconCoffee,
  compass: IconCompass,
  crown: IconCrown,
  'device-gamepad-2': IconDeviceGamepad2,
  diamond: IconDiamond,
  flame: IconFlame,
  flask: IconFlask,
  flower: IconFlower,
  folder: IconFolder,
  ghost: IconGhost,
  heart: IconHeart,
  leaf: IconLeaf,
  map: IconMap,
  'mood-smile': IconMoodSmile,
  moon: IconMoon,
  movie: IconMovie,
  music: IconMusic,
  notebook: IconNotebook,
  palette: IconPalette,
  paw: IconPaw,
  planet: IconPlanet,
  puzzle: IconPuzzle,
  robot: IconRobot,
  rocket: IconRocket,
  school: IconSchool,
  shield: IconShield,
  skull: IconSkull,
  star: IconStar,
  sun: IconSun,
  sword: IconSword,
  world: IconWorld
};

/** The names in FOLDER_ICONS, in the server's order. */
export const FOLDER_ICON_NAMES: readonly string[] = Object.keys(FOLDER_ICONS);

/** The icon a folder without a usable one of its own shows. */
export const DEFAULT_FOLDER_ICON = 'folder';

/** The icon a stored name draws as: itself when offered, the default otherwise. */
export function shownFolderIcon(name: string | undefined): string {
  return name !== undefined && Object.hasOwn(FOLDER_ICONS, name) ? name : DEFAULT_FOLDER_ICON;
}

/** The component for a name shownFolderIcon returned. */
export function folderIconComponent(shown: string): Component {
  return FOLDER_ICONS[shown] ?? FOLDER_ICONS[DEFAULT_FOLDER_ICON];
}
