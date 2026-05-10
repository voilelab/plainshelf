<template>
  <nav class="sidebar-nav-list" aria-label="Layers">
    <div class="sidebar-nav-item" :class="{ active: !selected }">
      <span class="tree-toggle-placeholder" aria-hidden="true"></span>
      <button type="button" class="sidebar-nav-item-label" @click="emit('select', '')">
        All books
      </button>
    </div>

    <LayerNodeItem
      v-for="node in nodes"
      :key="node.path"
      :node="node"
      :selected="selected"
      :deleting-map="deletingMap"
      :expanded-map="expandedMap"
      :depth="0"
      @toggle="toggleExpanded"
      @select="(path) => emit('select', path)"
      @move-book="(payload) => emit('move-book', payload)"
      @delete-layer="(path) => emit('delete-layer', path)"
    />
  </nav>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import LayerNodeItem from './LayerNodeItem.vue';

type LayerNode = {
  name: string;
  path: string;
  children: LayerNode[];
};

const props = defineProps<{
  nodes: LayerNode[];
  selected: string | undefined;
  deletingMap?: Record<string, boolean>;
}>();

const emit = defineEmits<{
  select: [path: string];
  'move-book': [payload: { bookId: string; targetLayer: string }];
  'delete-layer': [path: string];
}>();

const expandedMap = ref<Record<string, boolean>>({});

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
</style>
