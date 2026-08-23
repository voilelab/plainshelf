<template>
  <nav class="sidebar-nav-list" :aria-label="t('layout.layersNavLabel')">
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

    <TreeRoot
      v-slot="{ flattenItems }"
      class="layer-tree-root"
      :items="nodes"
      :get-key="(node: LayerNode) => node.path"
      :get-children="getChildren"
      :model-value="selectedTreeNode"
      v-model:expanded="expanded"
    >
      <ContextMenuRoot v-for="item in flattenItems" :key="item._id">
        <ContextMenuTrigger as-child :disabled="readOnly || !canManageLayer(item.value)">
          <TreeItem
            v-slot="{ isExpanded, handleToggle }"
            v-bind="item.bind"
            as="div"
            class="sidebar-nav-item layer-node"
            :class="{ active: isSelected(item.value), 'drop-target': dropTargetPath === item.value.path }"
            :style="{ paddingLeft: `calc(8px + ${(item.level - 1) * 14}px)` }"
            :draggable="canDragLayer(item.value)"
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
              :aria-label="isExpanded ? 'Collapse layer' : 'Expand layer'"
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
            <span class="sidebar-nav-count">{{ layerBookCount(item.value) }}</span>
          </TreeItem>
        </ContextMenuTrigger>
        <ContextMenuPortal>
          <ContextMenuContent class="reka-menu">
            <ContextMenuItem
              v-if="props.canOpenLayerFolder && canManageLayer(item.value)"
              class="reka-menu-item"
              @select="emit('open-layer-folder', item.value.path)"
            >{{ t('layout.openLayerFolder.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="props.canTransferLayer && canManageLayer(item.value)"
              class="reka-menu-item"
              @select="emit('transfer-layer', item.value.path)"
            >{{ t('layout.transferLayer.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              class="reka-menu-item"
              @select="emit('rename-layer', item.value.path)"
            >{{ t('layout.renameLayer.shortAction') }}</ContextMenuItem>
            <ContextMenuItem
              v-if="showDeleteButton(item.value)"
              class="reka-menu-item danger"
              :disabled="isDeleting(item.value)"
              @select="onDeleteLayer(item.value)"
            >{{ t('layout.deleteLayer.shortAction') }}</ContextMenuItem>
          </ContextMenuContent>
        </ContextMenuPortal>
      </ContextMenuRoot>
    </TreeRoot>

    <div
      v-if="dragLayer"
      class="layer-drag-preview"
      :style="{ transform: `translate3d(${dragLayer.x + 12}px, ${dragLayer.y + 12}px, 0)` }"
      aria-hidden="true"
    >
      {{ dragLayer.layerName }}
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
import { getLayerPath } from '@/utils/layers';

type LayerNode = {
  name: string;
  path: string;
  children: LayerNode[];
};

const props = defineProps<{
  nodes: LayerNode[];
  selected: string | undefined;
  deletingMap?: Record<string, boolean>;
  readOnly?: boolean;
  canOpenLayerFolder?: boolean;
  canTransferLayer?: boolean;
}>();

const emit = defineEmits<{
  select: [path: string];
  'move-book': [payload: { bookIds: string[]; targetLayer: string; batch: boolean }];
  'delete-layer': [path: string];
  'rename-layer': [path: string];
  'open-layer-folder': [path: string];
  'transfer-layer': [path: string];
  'move-layer': [payload: { layerPath: string; targetLayer: string }];
}>();

const { t } = useI18n();
const { books } = useBookStore();

const totalBookCount = computed(() => books.value.length);

const bookCountByLayer = computed(() => {
  const counts = new Map<string, number>();
  for (const book of books.value) {
    const layer = getLayerPath(book);
    counts.set(layer, (counts.get(layer) ?? 0) + 1);
  }
  return counts;
});

/**
 * A leaf's `children` is always an empty array (never undefined), but Reka's
 * TreeItem treats "children is an array" (even empty) as "has children" for
 * `aria-expanded` purposes. Returning `undefined` for empty arrays keeps
 * leaf nodes free of `aria-expanded`, matching the original markup.
 */
function getChildren(node: LayerNode): LayerNode[] | undefined {
  return node.children.length > 0 ? node.children : undefined;
}

function hasChildren(node: LayerNode): boolean {
  return node.children.length > 0;
}

function isSelected(node: LayerNode): boolean {
  return node.path === props.selected;
}

function findNodeByPath(nodes: LayerNode[], path: string): LayerNode | undefined {
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
 * Drive Reka's selection model from the app's selected layer so
 * aria-selected always matches the actual navigation state. Label clicks
 * bypass TreeItem's built-in select (@click.stop), and keyboard selection
 * would otherwise leave Reka's uncontrolled model stale.
 */
const selectedTreeNode = computed(() =>
  props.selected ? findNodeByPath(props.nodes, props.selected) : undefined
);

function layerBookCount(node: LayerNode): number {
  return bookCountByLayer.value.get(node.path) ?? 0;
}

function canManageLayer(node: LayerNode): boolean {
  return node.path !== '/';
}

function showDeleteButton(node: LayerNode): boolean {
  return canManageLayer(node) && !props.readOnly && layerBookCount(node) === 0;
}

function canDragLayer(node: LayerNode): boolean {
  return canManageLayer(node) && !props.readOnly;
}

function isDeleting(node: LayerNode): boolean {
  return props.deletingMap?.[node.path] ?? false;
}

function onDeleteLayer(node: LayerNode): void {
  if (isDeleting(node)) {
    return;
  }

  emit('delete-layer', node.path);
}

const expanded = ref<string[]>([]);
const isRootDropTarget = ref(false);
const dropTargetPath = ref<string | null>(null);
const dragLayer = ref<{ layerPath: string; layerName: string; x: number; y: number } | null>(null);

function startDragLayer(payload: { layerPath: string; layerName: string; x: number; y: number }): void {
  dragLayer.value = payload;
}

function moveDragLayer(payload: { x: number; y: number }): void {
  if (!dragLayer.value) {
    return;
  }

  dragLayer.value = { ...dragLayer.value, ...payload };
}

function endDragLayer(): void {
  dragLayer.value = null;
  isRootDropTarget.value = false;
}

function emitPointerPosition(event: DragEvent): void {
  if (event.clientX === 0 && event.clientY === 0) {
    return;
  }
  moveDragLayer({ x: event.clientX, y: event.clientY });
}

function onDrag(event: DragEvent): void {
  emitPointerPosition(event);
}

function onDragStart(event: DragEvent, node: LayerNode): void {
  if (!canDragLayer(node)) {
    event.preventDefault();
    return;
  }

  event.dataTransfer?.setData('application/x-plainshelf-layer-path', node.path);
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
  }
  startDragLayer({ layerPath: node.path, layerName: node.name, x: event.clientX, y: event.clientY });
}

function onDragEnd(node: LayerNode): void {
  if (dropTargetPath.value === node.path) {
    dropTargetPath.value = null;
  }
  endDragLayer();
}

function onDragEnter(node: LayerNode): void {
  dropTargetPath.value = node.path;
}

function onDragLeave(event: DragEvent, node: LayerNode): void {
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (!(currentTarget instanceof Node) || (relatedTarget instanceof Node && currentTarget.contains(relatedTarget))) {
    return;
  }
  if (dropTargetPath.value === node.path) {
    dropTargetPath.value = null;
  }
}

function onDrop(event: DragEvent, node: LayerNode): void {
  if (dropTargetPath.value === node.path) {
    dropTargetPath.value = null;
  }
  endDragLayer();

  if (props.readOnly) {
    return;
  }

  const draggedBooks = readDraggedBookIDs(event.dataTransfer);
  if (draggedBooks.ids.length > 0) {
    emit('move-book', { bookIds: draggedBooks.ids, targetLayer: node.path, batch: draggedBooks.batch });
    return;
  }

  const layerPath = event.dataTransfer?.getData('application/x-plainshelf-layer-path');
  if (!layerPath || layerPath === node.path || node.path.startsWith(`${layerPath}/`)) {
    return;
  }
  emit('move-layer', { layerPath, targetLayer: node.path });
}

function onRootDragOver(event: DragEvent): void {
  if (event.clientX !== 0 || event.clientY !== 0) {
    moveDragLayer({ x: event.clientX, y: event.clientY });
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
    emit('move-book', { bookIds: draggedBooks.ids, targetLayer: '/', batch: draggedBooks.batch });
    endDragLayer();
    return;
  }

  const layerPath = event.dataTransfer?.getData('application/x-plainshelf-layer-path');
  if (layerPath) {
    emit('move-layer', { layerPath, targetLayer: '/' });
  }
  endDragLayer();
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
.layer-tree-root {
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

.layer-node {
  gap: 4px;
  padding-right: 4px;
}

.layer-node :deep(.sidebar-nav-item-label) {
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

.layer-drag-preview {
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
