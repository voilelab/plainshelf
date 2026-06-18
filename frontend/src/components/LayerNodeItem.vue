<template>
  <div>
    <div
      class="sidebar-nav-item layer-node"
      :class="{ active: isSelected, 'drop-target': isDropTarget }"
      :style="{ paddingLeft: `calc(8px + ${depth * 14}px)` }"
      :draggable="canDragLayer"
      @dragstart="onDragStart"
      @dragover.prevent
      @dragenter.prevent="onDragEnter"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <button
        v-if="hasChildren"
        type="button"
        class="tree-toggle"
        :aria-label="isExpanded ? 'Collapse layer' : 'Expand layer'"
        @click.stop="emit('toggle', node.path)"
      >
        {{ isExpanded ? '▼' : '▶' }}
      </button>
      <span v-else class="tree-toggle-placeholder" aria-hidden="true"></span>

      <button type="button" class="sidebar-nav-item-label" @click="emit('select', node.path)">
        {{ node.name }}
      </button>
      <span class="sidebar-nav-count">{{ layerBookCount }}</span>
      <button
        v-if="canManageLayer && !readOnly"
        type="button"
        class="layer-action-btn"
        :title="t('layout.renameLayer.action')"
        :aria-label="t('layout.renameLayer.action')"
        @click.stop="onRenameLayer"
      >
        {{ t('layout.renameLayer.shortAction') }}
      </button>
      <button
        v-if="showDeleteButton"
        type="button"
        class="layer-action-btn"
        :title="t('layout.deleteLayer.action')"
        :aria-label="t('layout.deleteLayer.action')"
        :disabled="isDeleting"
        @click.stop="onDeleteLayer"
      >
        {{ t('layout.deleteLayer.shortAction') }}
      </button>
    </div>

    <div v-if="hasChildren && isExpanded" class="tree-children">
      <LayerNodeItem
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :selected="selected"
        :deleting-map="deletingMap"
        :expanded-map="expandedMap"
        :depth="depth + 1"
        :book-count-by-layer="bookCountByLayer"
        :read-only="readOnly"
        @toggle="(path) => emit('toggle', path)"
        @select="(path) => emit('select', path)"
        @move-book="(payload) => emit('move-book', payload)"
        @delete-layer="(path) => emit('delete-layer', path)"
        @rename-layer="(path) => emit('rename-layer', path)"
        @move-layer="(payload) => emit('move-layer', payload)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from '../i18n';

defineOptions({ name: 'LayerNodeItem' });

type LayerNode = {
  name: string;
  path: string;
  children: LayerNode[];
};

const props = defineProps<{
  node: LayerNode;
  selected: string | undefined;
  deletingMap?: Record<string, boolean>;
  expandedMap: Record<string, boolean>;
  depth: number;
  bookCountByLayer?: Map<string, number>;
  readOnly?: boolean;
}>();

const emit = defineEmits<{
  toggle: [path: string];
  select: [path: string];
  'move-book': [payload: { bookId: string; targetLayer: string }];
  'delete-layer': [path: string];
  'rename-layer': [path: string];
  'move-layer': [payload: { layerPath: string; targetLayer: string }];
}>();

const { t } = useI18n();

const hasChildren = computed(() => props.node.children.length > 0);
const isExpanded = computed(() => props.expandedMap[props.node.path] ?? false);
const isSelected = computed(() => props.node.path === props.selected);
const layerBookCount = computed(() => props.bookCountByLayer?.get(props.node.path) ?? 0);
const canManageLayer = computed(() => props.node.path !== '/');
const showDeleteButton = computed(() => canManageLayer.value && !props.readOnly && layerBookCount.value === 0);
const canDragLayer = computed(() => canManageLayer.value && !props.readOnly);
const isDeleting = computed(() => props.deletingMap?.[props.node.path] ?? false);
const isDropTarget = ref(false);

function onDeleteLayer(): void {
  if (isDeleting.value) {
    return;
  }

  emit('delete-layer', props.node.path);
}

function onRenameLayer(): void {
  emit('rename-layer', props.node.path);
}

function onDragStart(event: DragEvent): void {
  if (!canDragLayer.value) {
    event.preventDefault();
    return;
  }

  event.dataTransfer?.setData('application/x-plainshelf-layer-path', props.node.path);
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
  }
}

function onDragEnter(): void {
  isDropTarget.value = true;
}

function onDragLeave(event: DragEvent): void {
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (!(currentTarget instanceof Node) || (relatedTarget instanceof Node && currentTarget.contains(relatedTarget))) {
    return;
  }
  isDropTarget.value = false;
}

function onDrop(event: DragEvent): void {
  isDropTarget.value = false;
  if (props.readOnly) {
    return;
  }

  const bookId = event.dataTransfer?.getData('application/x-plainshelf-book-id');
  if (bookId) {
    emit('move-book', { bookId, targetLayer: props.node.path });
    return;
  }

  const layerPath = event.dataTransfer?.getData('application/x-plainshelf-layer-path');
  if (!layerPath || layerPath === props.node.path || props.node.path.startsWith(`${layerPath}/`)) {
    return;
  }
  emit('move-layer', { layerPath, targetLayer: props.node.path });
}
</script>

<style scoped>
.layer-node {
  gap: 4px;
  padding-right: 4px;
}

.layer-node :deep(.sidebar-nav-item-label) {
  flex: 1;
  min-width: 0;
  width: auto;
}

.layer-node.drop-target {
  background: #dbeafe;
  outline: 1px solid #93c5fd;
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

.tree-children {
  display: block;
}

.layer-action-btn {
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: #94a3b8;
  cursor: pointer;
  flex: 0 0 auto;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  padding: 3px 6px;
  transition: color 0.15s ease, border-color 0.15s ease, background-color 0.15s ease;
}

.layer-action-btn:hover,
.layer-action-btn:focus-visible {
  background: #fff1f2;
  border-color: #fecdd3;
  color: #b91c1c;
}

.layer-action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
