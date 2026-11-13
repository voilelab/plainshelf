<template>
  <nav class="sidebar-nav-list" :aria-label="t('layout.foldersNavLabel')">
    <p v-if="orderError" role="alert">{{ orderError }}</p>
    <ContextMenuRoot>
      <ContextMenuTrigger as-child :disabled="readOnly">
        <div
          class="sidebar-nav-item"
          :class="{ active: !selected, 'drop-target': isRootDropTarget }"
          @dragover.prevent="onRootDragOver"
          @dragenter.prevent="onRootDragEnter"
          @dragleave="onRootDragLeave"
          @drop="onRootDrop"
        >
          <span class="tree-toggle-placeholder" aria-hidden="true"></span>
          <button type="button" class="sidebar-nav-item-label" @click="emit('select', '')">
            {{ t('library.allBooks') }}
          </button>
          <span class="sidebar-nav-count">{{ totalBookCount }}</span>
        </div>
      </ContextMenuTrigger>
      <ContextMenuPortal>
        <ContextMenuContent class="reka-menu">
          <ContextMenuItem class="reka-menu-item" @select="emit('create-folder', '/')">
            {{ t('layout.createFolder.add') }}
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenuPortal>
    </ContextMenuRoot>
    <div
      v-if="topLevelBookCount > 0 || selected === ROOT_FOLDER_FILTER"
      class="sidebar-nav-item"
      :class="{ active: selected === ROOT_FOLDER_FILTER }"
    >
      <span class="tree-toggle-placeholder" aria-hidden="true"></span>
      <button type="button" class="sidebar-nav-item-label" @click="emit('select', ROOT_FOLDER_FILTER)">
        {{ t('bookCollection.noFolder') }}
      </button>
      <span class="sidebar-nav-count">{{ topLevelBookCount }}</span>
    </div>

    <TreeRoot
      v-slot="{ flattenItems }"
      class="folder-tree-root"
      :items="nodes"
      :get-key="(node: FolderNode) => node.path"
      :get-children="getChildren"
      :model-value="selectedTreeNode"
      v-model:expanded="expanded"
    >
      <ContextMenuRoot v-for="item in flattenItems" :key="item._id">
        <ContextMenuTrigger as-child :disabled="readOnly && !canTransferFolder">
          <TreeItem
            v-slot="{ isExpanded, handleToggle }"
            v-bind="item.bind"
            as="div"
            class="sidebar-nav-item folder-node"
            :class="[{ active: isSelected(item.value) }, dropClass(item.value)]"
            :style="{
              paddingLeft: `calc(8px + ${(item.level - 1) * 14}px)`,
              '--drop-indent': `calc(32px + ${(item.level - 1) * 14}px)`
            }"
            :draggable="canDragFolder()"
            @dragstart="(event: DragEvent) => onDragStart(event, item.value)"
            @drag="onDrag"
            @dragend="() => onDragEnd(item.value)"
            @dragover.prevent="(event: DragEvent) => onDragOver(event, item.value)"
            @dragenter.prevent="(event: DragEvent) => onDragOver(event, item.value)"
            @dragleave="(event: DragEvent) => onDragLeave(event, item.value)"
            @drop="(event: DragEvent) => onDrop(event, item.value)"
            @select="() => emit('select', item.value.path)"
          >
            <button
              v-if="hasChildren(item.value)"
              type="button"
              class="tree-toggle"
              :aria-label="isExpanded ? 'Collapse folder' : 'Expand folder'"
              @click.stop="handleToggle"
            >
              {{ isExpanded ? '▼' : '▶' }}
            </button>
            <span v-else class="tree-toggle-placeholder" aria-hidden="true"></span>

            <FolderIcon :icon="folderIcon(item.value)" />
            <button
              type="button"
              class="sidebar-nav-item-label"
              @click.stop="emit('select', item.value.path)"
            >
              {{ item.value.name }}
            </button>
            <span class="sidebar-nav-count">{{ folderBookCount(item.value) }}</span>
          </TreeItem>
        </ContextMenuTrigger>
        <ContextMenuPortal>
          <ContextMenuContent class="reka-menu">
            <ContextMenuItem
              v-if="!readOnly"
              class="reka-menu-item"
              @select="emit('create-folder', item.value.path)"
            >
              {{ t('layout.createFolder.add') }}
            </ContextMenuItem>
            <ContextMenuItem
              v-if="props.canOpenFolder"
              class="reka-menu-item"
              @select="emit('open-folder', item.value.path)"
            >{{ t('layout.openFolder.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="props.canTransferFolder"
              class="reka-menu-item"
              @select="emit('transfer-folder', item.value.path)"
            >{{ t('layout.transferFolder.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="!readOnly"
              class="reka-menu-item"
              @select="emit('rename-folder', item.value.path)"
            >{{ t('layout.renameFolder.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="canChangeIcon(item.value)"
              class="reka-menu-item"
              @select="emit('change-icon', item.value.path)"
            >{{ t('layout.folderIcon.shortAction') }}</ContextMenuItem>
            <template v-if="!readOnly && siblingsOf(item.value.path).length > 1">
              <ContextMenuSeparator class="reka-menu-separator" />
              <ContextMenuItem
                class="reka-menu-item"
                :disabled="savingOrder || !orderNeighbor(item.value.path, -1)"
                @select="stepOrder(item.value.path, -1)"
              >{{ t('layout.folderOrder.up') }}</ContextMenuItem>
              <ContextMenuItem
                class="reka-menu-item"
                :disabled="savingOrder || !orderNeighbor(item.value.path, 1)"
                @select="stepOrder(item.value.path, 1)"
              >{{ t('layout.folderOrder.down') }}</ContextMenuItem>
            </template>
            <ContextMenuItem
              v-if="showDeleteButton(item.value)"
              class="reka-menu-item danger"
              :disabled="isDeleting(item.value)"
              @select="onDeleteFolder(item.value)"
            >{{ t('layout.deleteFolder.shortAction') }}</ContextMenuItem>
          </ContextMenuContent>
        </ContextMenuPortal>
      </ContextMenuRoot>
    </TreeRoot>

    <div
      v-if="dragFolder"
      class="folder-drag-preview"
      :style="{ transform: `translate3d(${dragFolder.x + 12}px, ${dragFolder.y + 12}px, 0)` }"
      aria-hidden="true"
    >
      {{ dragFolder.folderName }}
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuSeparator,
  ContextMenuTrigger,
  TreeItem,
  TreeRoot
} from 'reka-ui';
import FolderIcon from './FolderIcon.vue';
import type { FolderMeta } from '@/api/folders';
import { useBookStore } from '@/composables/useBookStore';
import { useI18n } from '@/i18n';
import { getFolderPath, ROOT_FOLDER_FILTER } from '@/utils/folders';
import { planFolderOrder, type FolderOrderPlacement } from '@/utils/folderOrder';
import { setFolderSortKey } from '@/api/folders';
import { getActiveShelfID } from '@/api/client';
import { useFolderStore } from '@/composables/useFolderStore';

type FolderNode = {
  name: string;
  path: string;
  children: FolderNode[];
};

const props = defineProps<{
  nodes: FolderNode[];
  selected: string | undefined;
  deletingMap?: Record<string, boolean>;
  readOnly?: boolean;
  canOpenFolder?: boolean;
  canTransferFolder?: boolean;
  folderMeta?: Record<string, FolderMeta>;
}>();

const emit = defineEmits<{
  select: [path: string];
  'create-folder': [parentPath: string];
  'move-book': [payload: { bookIds: string[]; targetFolder: string; batch: boolean }];
  'delete-folder': [path: string];
  'rename-folder': [path: string];
  'open-folder': [path: string];
  'transfer-folder': [path: string];
  'move-folder': [payload: { folderPath: string; targetFolder: string }];
  'change-icon': [path: string];
}>();

const { t } = useI18n();
const { folderMeta, folderMetaError, fetchFolders, error: folderLoadError } = useFolderStore();
const savingOrder = ref(false);
const orderError = ref('');
const FOLDER_MIME = 'application/x-plainshelf-folder-path';
const REORDER_EDGE = 0.25;
type FolderDropPlacement = FolderOrderPlacement | 'into';
function parentOf(path: string): string {
  return path.split('/').slice(0, -1).join('/');
}
function siblingsOf(path: string): string[] {
  const parent = parentOf(path);
  const nodes = parent
    ? (findNodeByPath(props.nodes, parent)?.children ?? [])
    : props.nodes;
  return nodes.map((node) => node.path);
}
function orderNeighbor(path: string, direction: number): string | undefined {
  const siblings = siblingsOf(path);
  return siblings[siblings.indexOf(path) + direction];
}
function stepOrder(path: string, direction: number): void {
  const neighbor = orderNeighbor(path, direction);
  if (!neighbor) return;
  void saveOrder(path, neighbor, direction < 0 ? 'before' : 'after');
}
async function saveOrder(
  source: string,
  target: string,
  placement: FolderOrderPlacement
): Promise<void> {
  if (props.readOnly || savingOrder.value) return;
  const shelfID = getActiveShelfID();
  const plan = planFolderOrder(
    siblingsOf(target),
    source,
    target,
    folderMeta.value,
    placement
  );
  if (!plan.length) return;
  savingOrder.value = true;
  orderError.value = '';
  try {
    for (const change of plan) {
      if (props.readOnly || shelfID !== getActiveShelfID())
        throw new Error('Shelf changed');
      await setFolderSortKey(change.path, change.key, change.version, shelfID);
    }
  } catch {
    orderError.value = t('layout.folderOrder.failed');
  } finally {
    await fetchFolders();
    if (folderLoadError.value || folderMetaError.value)
      orderError.value = t('layout.folderOrder.reloadFailed');
    savingOrder.value = false;
  }
}
const { books } = useBookStore();

const totalBookCount = computed(() => books.value.length);
const topLevelBookCount = computed(() => bookCountByFolder.value.get('') ?? 0);

const bookCountByFolder = computed(() => {
  const counts = new Map<string, number>();
  for (const book of books.value) {
    const folder = getFolderPath(book);
    counts.set(folder, (counts.get(folder) ?? 0) + 1);
  }
  return counts;
});

// Reka UI 2.10.5 TreeItem treats an empty children array as expandable; leaves require undefined.
function getChildren(node: FolderNode): FolderNode[] | undefined {
  return node.children.length > 0 ? node.children : undefined;
}

function hasChildren(node: FolderNode): boolean {
  return node.children.length > 0;
}

function isSelected(node: FolderNode): boolean {
  return node.path === props.selected;
}

function findNodeByPath(nodes: FolderNode[], path: string): FolderNode | undefined {
  for (const node of nodes) {
    if (node.path === path) {
      return node;
    }
    const found = findNodeByPath(node.children, path);
    if (found) {
      return found;
    }
  }
  return undefined;
}

const selectedTreeNode = computed(() =>
  props.selected ? findNodeByPath(props.nodes, props.selected) : undefined
);

function folderBookCount(node: FolderNode): number {
  return bookCountByFolder.value.get(node.path) ?? 0;
}

function folderIcon(node: FolderNode): string {
  return props.folderMeta?.[node.path]?.icon ?? '';
}

function canChangeIcon(node: FolderNode): boolean {
  return !props.readOnly && !props.folderMeta?.[node.path]?.readOnly;
}

function showDeleteButton(node: FolderNode): boolean {
  return !props.readOnly && folderBookCount(node) === 0 && node.children.length === 0;
}

function canDragFolder(): boolean {
  return !props.readOnly && !savingOrder.value;
}

function isDeleting(node: FolderNode): boolean {
  return props.deletingMap?.[node.path] ?? false;
}

function onDeleteFolder(node: FolderNode): void {
  if (isDeleting(node)) {
    return;
  }

  emit('delete-folder', node.path);
}

const expanded = ref<string[]>([]);
const isRootDropTarget = ref(false);
const dropIndicator = ref<{ path: string; placement: FolderDropPlacement } | null>(null);

function dropClass(node: FolderNode): string | undefined {
  if (dropIndicator.value?.path !== node.path) return undefined;
  return dropIndicator.value.placement === 'into' ? 'drop-target' : `drop-${dropIndicator.value.placement}`;
}

function dropPlacement(event: DragEvent, node: FolderNode, source: string | undefined): FolderDropPlacement {
  if (!source || source === node.path || parentOf(source) !== parentOf(node.path)) return 'into';
  if (!(event.currentTarget instanceof Element)) return 'into';
  const rect = event.currentTarget.getBoundingClientRect();
  if (rect.height <= 0) return 'into';
  const offset = (event.clientY - rect.top) / rect.height;
  if (offset < REORDER_EDGE) return 'before';
  const showsChildren = hasChildren(node) && expanded.value.includes(node.path);
  if (offset > 1 - REORDER_EDGE && !showsChildren) return 'after';
  return 'into';
}
const dragFolder = ref<{ folderPath: string; folderName: string; x: number; y: number } | null>(null);

function startDragFolder(payload: { folderPath: string; folderName: string; x: number; y: number }): void {
  dragFolder.value = payload;
}

function moveDragFolder(payload: { x: number; y: number }): void {
  if (!dragFolder.value) {
    return;
  }

  dragFolder.value = { ...dragFolder.value, ...payload };
}

function endDragFolder(): void {
  dragFolder.value = null;
  isRootDropTarget.value = false;
}

function emitPointerPosition(event: DragEvent): void {
  if (event.clientX === 0 && event.clientY === 0) {
    return;
  }
  moveDragFolder({ x: event.clientX, y: event.clientY });
}

function onDrag(event: DragEvent): void {
  emitPointerPosition(event);
}

function onDragStart(event: DragEvent, node: FolderNode): void {
  if (!canDragFolder()) {
    event.preventDefault();
    return;
  }

  event.dataTransfer?.setData(FOLDER_MIME, node.path);
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
  }
  startDragFolder({
    folderPath: node.path,
    folderName: node.name,
    x: event.clientX,
    y: event.clientY
  });
}

function onDragEnd(node: FolderNode): void {
  if (dropIndicator.value?.path === node.path) {
    dropIndicator.value = null;
  }
  endDragFolder();
}

function onDragOver(event: DragEvent, node: FolderNode): void {
  emitPointerPosition(event);
  dropIndicator.value = {
    path: node.path,
    placement: dropPlacement(event, node, dragFolder.value?.folderPath)
  };
}

function onDragLeave(event: DragEvent, node: FolderNode): void {
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (!(currentTarget instanceof Node) || (relatedTarget instanceof Node && currentTarget.contains(relatedTarget))) {
    return;
  }
  if (dropIndicator.value?.path === node.path) {
    dropIndicator.value = null;
  }
}

function onDrop(event: DragEvent, node: FolderNode): void {
  dropIndicator.value = null;
  endDragFolder();

  if (props.readOnly) {
    return;
  }

  const folderPath = event.dataTransfer?.getData(FOLDER_MIME);
  const placement = dropPlacement(event, node, folderPath);
  if (folderPath && placement !== 'into') {
    void saveOrder(folderPath, node.path, placement);
    return;
  }
  const draggedBooks = readDraggedBookIDs(event.dataTransfer);
  if (draggedBooks.ids.length > 0) {
    emit('move-book', {
      bookIds: draggedBooks.ids,
      targetFolder: node.path,
      batch: draggedBooks.batch
    });
    return;
  }

  if (
    !folderPath ||
    folderPath === node.path ||
    node.path.startsWith(`${folderPath}/`)
  ) {
    return;
  }
  emit('move-folder', { folderPath, targetFolder: node.path });
}

function onRootDragOver(event: DragEvent): void {
  if (event.clientX !== 0 || event.clientY !== 0) {
    moveDragFolder({ x: event.clientX, y: event.clientY });
  }
}

function onRootDragEnter(): void {
  isRootDropTarget.value = true;
}

function onRootDragLeave(event: DragEvent): void {
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (!(currentTarget instanceof Node) || (relatedTarget instanceof Node && currentTarget.contains(relatedTarget))) {
    return;
  }
  isRootDropTarget.value = false;
}

function onRootDrop(event: DragEvent): void {
  isRootDropTarget.value = false;

  if (props.readOnly) {
    endDragFolder();
    return;
  }
  const draggedBooks = readDraggedBookIDs(event.dataTransfer);
  if (draggedBooks.ids.length > 0) {
    emit('move-book', {
      bookIds: draggedBooks.ids,
      targetFolder: '/',
      batch: draggedBooks.batch
    });
    endDragFolder();
    return;
  }

  const folderPath = event.dataTransfer?.getData(FOLDER_MIME);
  if (folderPath) {
    emit('move-folder', { folderPath, targetFolder: '/' });
  }
  endDragFolder();
}

function readDraggedBookIDs(dataTransfer: DataTransfer | null): { ids: string[]; batch: boolean } {
  if (!dataTransfer) return { ids: [], batch: false };
  const raw = dataTransfer.getData('application/x-plainshelf-book-ids');
  if (raw) {
    try {
      const ids = JSON.parse(raw);
      if (Array.isArray(ids)) return { ids: [...new Set(ids.filter((id): id is string => typeof id === 'string' && id.length > 0))], batch: true };
    } catch {
    }
  }
  const single = dataTransfer.getData('application/x-plainshelf-book-id');
  return { ids: single ? [single] : [], batch: false };
}

function expandPath(path: string | undefined): void {
  if (!path) {
    return;
  }

  const segments = path.split('/').filter(Boolean);
  const next = new Set(expanded.value);
  for (let i = 0; i < segments.length; i += 1) {
    next.add(segments.slice(0, i + 1).join('/'));
  }
  expanded.value = [...next];
}

watch(
  () => props.nodes,
  (nodes) => {
    const next = new Set(expanded.value);
    for (const node of nodes) {
      next.add(node.path);
    }
    expanded.value = [...next];
    expandPath(props.selected);
  },
  { immediate: true }
);

watch(
  () => props.selected,
  (path) => {
    expandPath(path);
  },
  { immediate: true }
);
</script>

<style scoped>

.folder-tree-root {
  display: flex;
  flex-direction: column;
  gap: 2px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.tree-toggle-placeholder {
  flex: 0 0 20px;
  width: 20px;
}

.drop-target {
  background: var(--accent-tint);
  outline: 1px solid var(--accent-outline);
}

.folder-node {
  gap: 4px;
  padding-right: 4px;
  position: relative;
}

.folder-node.drop-before::before,
.folder-node.drop-after::after {
  background: var(--accent);
  border-radius: 2px;
  content: '';
  height: 2px;
  left: var(--drop-indent);
  pointer-events: none;
  position: absolute;
  right: 4px;
}

.folder-node.drop-before::before {
  top: -2px;
}

.folder-node.drop-after::after {
  bottom: -2px;
}

.folder-node :deep(.sidebar-nav-item-label) {
  flex: 1;
  min-width: 0;
  width: auto;
}

.tree-toggle,
.tree-toggle-placeholder {
  align-items: center;
  border: 0;
  color: var(--text-subtle);
  display: inline-flex;
  flex: 0 0 20px;
  font-size: 11px;
  height: 20px;
  justify-content: center;
  width: 20px;
}

.tree-toggle {
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
}

.tree-toggle:hover {
  background: var(--accent-soft);
}

.folder-drag-preview {
  background: var(--tooltip-bg);
  border-radius: 8px;
  box-shadow: 0 10px 24px rgb(var(--shadow-rgb) / 0.22);
  color: var(--tooltip-text);
  font-size: 13px;
  font-weight: 700;
  left: 0;
  max-width: 220px;
  overflow: hidden;
  padding: 8px 10px;
  pointer-events: none;
  position: fixed;
  text-overflow: ellipsis;
  top: 0;
  white-space: nowrap;
  z-index: 2000;
}
</style>
