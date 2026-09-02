<template>
  <nav class="sidebar-nav-list" :aria-label="t('layout.foldersNavLabel')">
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
        <!-- The synthetic / node cannot be renamed, moved, or deleted, but it
             still owns top-level folder creation. -->
        <!-- Open on a read-only shelf only when it still holds something that
             shelf allows: copying the folder out. Every item below states its
             own condition, so the trigger is not what keeps writes off it. -->
        <ContextMenuTrigger as-child :disabled="readOnly && !canTransferFolder">
          <TreeItem
            v-slot="{ isExpanded, handleToggle }"
            v-bind="item.bind"
            as="div"
            class="sidebar-nav-item folder-node"
            :class="{ active: isSelected(item.value), 'drop-target': dropTargetPath === item.value.path }"
            :style="{ paddingLeft: `calc(8px + ${(item.level - 1) * 14}px)` }"
            :draggable="canDragFolder(item.value)"
            @dragstart="(event: DragEvent) => onDragStart(event, item.value)"
            @drag="onDrag"
            @dragend="() => onDragEnd(item.value)"
            @dragover.prevent="onDrag"
            @dragenter.prevent="() => onDragEnter(item.value)"
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
              v-if="props.canOpenFolder && canManageFolder(item.value)"
              class="reka-menu-item"
              @select="emit('open-folder', item.value.path)"
            >{{ t('layout.openFolder.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="props.canTransferFolder && canManageFolder(item.value)"
              class="reka-menu-item"
              @select="emit('transfer-folder', item.value.path)"
            >{{ t('layout.transferFolder.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="canManageFolder(item.value) && !readOnly"
              class="reka-menu-item"
              @select="emit('rename-folder', item.value.path)"
            >{{ t('layout.renameFolder.shortAction') }}</ContextMenuItem>
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
  ContextMenuTrigger,
  TreeItem,
  TreeRoot
} from 'reka-ui';
import { useBookStore } from '@/composables/useBookStore';
import { useI18n } from '@/i18n';
import { getFolderPath } from '@/utils/folders';

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
}>();

const { t } = useI18n();
const { books } = useBookStore();

const totalBookCount = computed(() => books.value.length);

const bookCountByFolder = computed(() => {
  const counts = new Map<string, number>();
  for (const book of books.value) {
    const folder = getFolderPath(book);
    counts.set(folder, (counts.get(folder) ?? 0) + 1);
  }
  return counts;
});

/**
 * A leaf's `children` is always an empty array (never undefined), but Reka's
 * TreeItem treats "children is an array" (even empty) as "has children" for
 * `aria-expanded` purposes. Returning `undefined` for empty arrays keeps
 * leaf nodes free of `aria-expanded`, matching the original markup.
 */
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

/**
 * Drive Reka's selection model from the app's selected folder so
 * aria-selected always matches the actual navigation state. Label clicks
 * bypass TreeItem's built-in select (@click.stop), and keyboard selection
 * would otherwise leave Reka's uncontrolled model stale.
 */
const selectedTreeNode = computed(() =>
  props.selected ? findNodeByPath(props.nodes, props.selected) : undefined
);

function folderBookCount(node: FolderNode): number {
  return bookCountByFolder.value.get(node.path) ?? 0;
}

function canManageFolder(node: FolderNode): boolean {
  return node.path !== '/';
}

function showDeleteButton(node: FolderNode): boolean {
  return canManageFolder(node) && !props.readOnly && folderBookCount(node) === 0;
}

function canDragFolder(node: FolderNode): boolean {
  return canManageFolder(node) && !props.readOnly;
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
const dropTargetPath = ref<string | null>(null);
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
  if (!canDragFolder(node)) {
    event.preventDefault();
    return;
  }

  event.dataTransfer?.setData('application/x-plainshelf-folder-path', node.path);
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
  }
  startDragFolder({ folderPath: node.path, folderName: node.name, x: event.clientX, y: event.clientY });
}

function onDragEnd(node: FolderNode): void {
  if (dropTargetPath.value === node.path) {
    dropTargetPath.value = null;
  }
  endDragFolder();
}

function onDragEnter(node: FolderNode): void {
  dropTargetPath.value = node.path;
}

function onDragLeave(event: DragEvent, node: FolderNode): void {
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (!(currentTarget instanceof Node) || (relatedTarget instanceof Node && currentTarget.contains(relatedTarget))) {
    return;
  }
  if (dropTargetPath.value === node.path) {
    dropTargetPath.value = null;
  }
}

function onDrop(event: DragEvent, node: FolderNode): void {
  if (dropTargetPath.value === node.path) {
    dropTargetPath.value = null;
  }
  endDragFolder();

  if (props.readOnly) {
    return;
  }

  const draggedBooks = readDraggedBookIDs(event.dataTransfer);
  if (draggedBooks.ids.length > 0) {
    emit('move-book', { bookIds: draggedBooks.ids, targetFolder: node.path, batch: draggedBooks.batch });
    return;
  }

  const folderPath = event.dataTransfer?.getData('application/x-plainshelf-folder-path');
  if (!folderPath || folderPath === node.path || node.path.startsWith(`${folderPath}/`)) {
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

  const draggedBooks = readDraggedBookIDs(event.dataTransfer);
  if (draggedBooks.ids.length > 0) {
    emit('move-book', { bookIds: draggedBooks.ids, targetFolder: '/', batch: draggedBooks.batch });
    endDragFolder();
    return;
  }

  const folderPath = event.dataTransfer?.getData('application/x-plainshelf-folder-path');
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
      // Fall back to the legacy single-book drag payload.
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
  background: #dbeafe;
  outline: 1px solid #93c5fd;
}

.folder-node {
  gap: 4px;
  padding-right: 4px;
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
  color: #5f6a7a;
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
  background: #e6edf8;
}

.folder-drag-preview {
  background: rgba(15, 23, 42, 0.92);
  border-radius: 8px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.22);
  color: #fff;
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
