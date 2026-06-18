<template>
  <nav class="sidebar-nav-list" aria-label="Layers">
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
        All books
      </button>
      <span class="sidebar-nav-count">{{ totalBookCount }}</span>
    </div>

    <LayerNodeItem
      v-for="node in nodes"
      :key="node.path"
      :node="node"
      :selected="selected"
      :deleting-map="deletingMap"
      :expanded-map="expandedMap"
      :depth="0"
      :book-count-by-layer="bookCountByLayer"
      :read-only="readOnly"
      @toggle="toggleExpanded"
      @select="(path) => emit('select', path)"
      @move-book="(payload) => emit('move-book', payload)"
      @delete-layer="(path) => emit('delete-layer', path)"
      @rename-layer="(path) => emit('rename-layer', path)"
      @move-layer="(payload) => emit('move-layer', payload)"
      @drag-layer-start="startDragLayer"
      @drag-layer-move="moveDragLayer"
      @drag-layer-end="endDragLayer"
    />

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
import LayerNodeItem from './LayerNodeItem.vue';
import { useBookStore } from '../composables/useBookStore';
import { getLayerPath } from '../utils/layers';

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
}>();

const emit = defineEmits<{
  select: [path: string];
  'move-book': [payload: { bookId: string; targetLayer: string }];
  'delete-layer': [path: string];
  'rename-layer': [path: string];
  'move-layer': [payload: { layerPath: string; targetLayer: string }];
}>();

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

const expandedMap = ref<Record<string, boolean>>({});
const isRootDropTarget = ref(false);
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

  const bookId = event.dataTransfer?.getData('application/x-plainshelf-book-id');
  if (bookId) {
    emit('move-book', { bookId, targetLayer: '/' });
    endDragLayer();
    return;
  }

  const layerPath = event.dataTransfer?.getData('application/x-plainshelf-layer-path');
  if (layerPath) {
    emit('move-layer', { layerPath, targetLayer: '/' });
  }
  endDragLayer();
}

function toggleExpanded(path: string): void {
  expandedMap.value[path] = !(expandedMap.value[path] ?? false);
}

function expandPath(path: string | undefined): void {
  if (!path) {
    return;
  }

  const segments = path.split('/').filter(Boolean);
  for (let i = 0; i < segments.length; i += 1) {
    const segmentPath = segments.slice(0, i + 1).join('/');
    expandedMap.value[segmentPath] = true;
  }
}

watch(
  () => props.nodes,
  (nodes) => {
    const nextExpanded = { ...expandedMap.value };
    for (const node of nodes) {
      nextExpanded[node.path] = true;
    }
    expandedMap.value = nextExpanded;
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
.tree-toggle-placeholder {
  flex: 0 0 20px;
  width: 20px;
}

.drop-target {
  background: #dbeafe;
  outline: 1px solid #93c5fd;
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
