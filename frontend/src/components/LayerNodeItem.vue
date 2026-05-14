<template>
  <div>
    <div
      class="sidebar-nav-item layer-node"
      :class="{ active: isSelected, 'drop-target': isDropTarget }"
      :style="{ paddingLeft: `calc(8px + ${depth * 14}px)` }"
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
      <span class="sidebar-nav-count">{{ bookCountByLayer?.get(node.path) ?? 0 }}</span>
      <button
        v-if="showDeleteButton"
        type="button"
        class="layer-delete-btn"
        title="Delete empty layer"
        aria-label="Delete empty layer"
        :disabled="isDeleting"
        @click.stop="onDeleteLayer"
      >
        Delete
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
        @toggle="(path) => emit('toggle', path)"
        @select="(path) => emit('select', path)"
        @move-book="(payload) => emit('move-book', payload)"
        @delete-layer="(path) => emit('delete-layer', path)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

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
}>();

const emit = defineEmits<{
  toggle: [path: string];
  select: [path: string];
  'move-book': [payload: { bookId: string; targetLayer: string }];
  'delete-layer': [path: string];
}>();

const hasChildren = computed(() => props.node.children.length > 0);
const isExpanded = computed(() => props.expandedMap[props.node.path] ?? false);
const isSelected = computed(() => props.node.path === props.selected);
const showDeleteButton = computed(() => props.node.path !== '/');
const isDeleting = computed(() => props.deletingMap?.[props.node.path] ?? false);
const isDropTarget = ref(false);

function onDeleteLayer(): void {
  if (isDeleting.value) {
    return;
  }

  const confirmed = window.confirm(
    `Delete empty layer "${props.node.path}"?\nThis will fail if the layer contains books or child layers.`
  );
  if (!confirmed) {
    return;
  }

  emit('delete-layer', props.node.path);
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
  const bookId = event.dataTransfer?.getData('application/x-txtlib-book-id');
  if (!bookId) {
    return;
  }
  emit('move-book', { bookId, targetLayer: props.node.path });
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

.layer-delete-btn {
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

.layer-delete-btn:hover,
.layer-delete-btn:focus-visible {
  background: #fff1f2;
  border-color: #fecdd3;
  color: #b91c1c;
}

.layer-delete-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
