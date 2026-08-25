<template>
  <nav class="folder-breadcrumb" :aria-label="t('bookDetail.folderPath')">
    <template v-for="(item, index) in breadcrumbItems" :key="`${item.path}-${index}`">
      <RouterLink class="breadcrumb-item" :to="item.to">{{ item.label }}</RouterLink>
      <span v-if="index < breadcrumbItems.length - 1" class="breadcrumb-separator" aria-hidden="true">
        /
      </span>
    </template>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink, type RouteLocationRaw } from 'vue-router';
import { booksRouteForFolderPath, normalizeFolderInput } from '@/utils/folders';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const props = defineProps<{
  folders?: string | string[] | null;
}>();

interface BreadcrumbItem {
  label: string;
  path: string;
  to: RouteLocationRaw;
}

const breadcrumbItems = computed<BreadcrumbItem[]>(() => {
  const segments = normalizeFolderInput(props.folders);
  const items: BreadcrumbItem[] = [
    {
      label: t('bookDetail.root'),
      path: '',
      to: booksRouteForFolderPath('')
    }
  ];

  for (let i = 0; i < segments.length; i += 1) {
    const folderPath = segments.slice(0, i + 1).join('/');
    items.push({
      label: segments[i],
      path: folderPath,
      to: booksRouteForFolderPath(folderPath)
    });
  }

  return items;
});
</script>

<style scoped>
.folder-breadcrumb {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  line-height: 1.5;
  color: color-mix(in srgb, var(--text) 58%, white);
}

.breadcrumb-item {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--panel) 76%, transparent);
  color: inherit;
  font-weight: 600;
  letter-spacing: 0.01em;
  cursor: pointer;
  text-decoration: none;
  transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;
}

.breadcrumb-item:hover {
  background: color-mix(in srgb, var(--panel) 92%, white);
  border-color: color-mix(in srgb, var(--accent) 20%, var(--border));
  color: color-mix(in srgb, var(--text) 72%, white);
}

.breadcrumb-separator {
  opacity: 0.48;
  font-weight: 700;
}

@media (max-width: 768px) {
  .breadcrumb-item {
    min-height: 44px;
  }
}
</style>
