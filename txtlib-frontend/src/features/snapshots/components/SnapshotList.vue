<template>
  <aside class="snapshot-list">
    <div class="snapshot-list-header">
      <h3>Snapshots</h3>
      <p class="meta">{{ snapshots.length }} total</p>
    </div>

    <div v-if="loading" class="loading list-status">Loading snapshots...</div>
    <div v-else-if="snapshots.length === 0" class="meta list-status">No snapshots yet.</div>

    <div v-else class="snapshot-items" role="list" aria-label="Book snapshots">
      <button
        v-for="snapshot in snapshots"
        :key="snapshot.id"
        type="button"
        class="snapshot-item"
        :class="{ active: snapshot.id === activeSnapshotId }"
        @click="$emit('select', snapshot.id)"
      >
        <div class="snapshot-item-top">
          <strong class="snapshot-id">{{ snapshot.id }}</strong>
          <span v-if="snapshot.id === currentSnapshotId" class="current-badge">Current</span>
        </div>
        <p class="meta snapshot-created">{{ formatTimestamp(snapshot.created_at) }}</p>
        <p class="meta snapshot-hash">md5: {{ shortHash(snapshot.md5_hash) }}</p>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import type { SnapshotMeta } from '../types';

defineProps<{
  snapshots: SnapshotMeta[];
  activeSnapshotId: string;
  currentSnapshotId?: string;
  loading?: boolean;
}>();

defineEmits<{
  select: [snapshotId: string];
}>();

function shortHash(hash: string): string {
  return (hash || '').slice(0, 8) || '-';
}

function formatTimestamp(value: string): string {
  if (!value) {
    return '-';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString();
}
</script>

<style scoped>
.snapshot-list {
  width: 300px;
  flex: 0 0 300px;
  min-width: 240px;
  max-width: 360px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  min-width: 0;
  box-sizing: border-box;
  overflow-y: auto;
  overflow-x: hidden;
  border-right: 1px solid var(--border);
  background: #fbfdff;
}

.snapshot-list-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 12px 6px;
}

.snapshot-list-header h3 {
  margin: 0;
  font-size: 16px;
}

.snapshot-items {
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: visible;
  min-height: 0;
  padding: 0 10px 12px;
}

.snapshot-item {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #fff;
  text-align: left;
  padding: 10px;
  cursor: pointer;
  display: grid;
  gap: 4px;
}

.snapshot-item:hover {
  background: #f8fbff;
}

.snapshot-item.active {
  border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 30%, transparent);
  background: #eef5ff;
}

.snapshot-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.snapshot-id {
  font-size: 14px;
}

.current-badge {
  font-size: 11px;
  font-weight: 700;
  color: #0f4fa8;
  background: #e1efff;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  padding: 2px 8px;
}

.snapshot-created,
.snapshot-hash {
  margin: 0;
}

.list-status {
  margin: 0;
}

@media (max-width: 900px) {
  .snapshot-list {
    width: 100%;
    flex: 0 0 auto;
    min-width: 0;
    max-width: none;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }

  .snapshot-items {
    max-height: 260px;
  }
}
</style>
