<template>
  <span
    v-if="state"
    class="book-download-badge"
    :class="`is-${state}`"
    :data-download-state="state"
  >
    <span class="book-download-dot" aria-hidden="true"></span>
    <span class="book-download-text">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { DownloadState } from '@/types/book';
import { useI18n } from '@/i18n';

// Rendered only when the book actually carries a download state. A server or
// desktop listing has no download concept and omits `download_state`, so those
// clients keep the row they had instead of gaining an empty marker; the mobile
// provider annotates every book (mobileBookshelfProvider.annotateDownloadState),
// so on the device every row is labelled.
const props = defineProps<{ state?: DownloadState }>();

const { t } = useI18n();

const LABEL_KEYS: Record<DownloadState, string> = {
  not_downloaded: 'bookCollection.downloadState.notDownloaded',
  downloaded: 'bookCollection.downloadState.downloaded',
  update_available: 'bookCollection.downloadState.updateAvailable',
  downloading: 'bookCollection.downloadState.downloading',
  failed: 'bookCollection.downloadState.failed'
};

const label = computed(() => (props.state ? t(LABEL_KEYS[props.state]) : ''));
</script>

<style scoped>
.book-download-badge {
  /* Every state below sets its own; this keeps the color-mix()es valid if one
     is ever added to DownloadState without a colour here. */
  --badge-color: var(--muted);
  align-items: center;
  background: color-mix(in srgb, var(--badge-color) 10%, white);
  border: 1px solid color-mix(in srgb, var(--badge-color) 35%, white);
  border-radius: 999px;
  color: color-mix(in srgb, var(--badge-color) 80%, var(--text));
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 11px;
  font-weight: 600;
  gap: 5px;
  line-height: 1.4;
  max-width: 100%;
  padding: 2px 8px;
  white-space: nowrap;
}

/* The dot repeats the state as a second, non-textual channel; the text beside
   it is what carries the meaning, so colour alone is never the difference. */
.book-download-dot {
  background: var(--badge-color);
  border-radius: 999px;
  flex: 0 0 auto;
  height: 6px;
  width: 6px;
}

.book-download-text {
  overflow: hidden;
  text-overflow: ellipsis;
}

.book-download-badge.is-not_downloaded { --badge-color: var(--muted); }
.book-download-badge.is-downloaded { --badge-color: #2f8f5b; }
.book-download-badge.is-update_available { --badge-color: #b26a00; }
.book-download-badge.is-downloading { --badge-color: var(--accent); }
.book-download-badge.is-failed { --badge-color: var(--danger); }
</style>
