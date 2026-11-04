<template>
  <p v-if="status === 'checking'" class="shelf-source-status" role="status">
    {{ t('shelfSource.checking') }}
  </p>
  <p v-else-if="status === 'ok'" class="shelf-source-status ok" role="status">
    <Icon name="check" class="shelf-source-status-icon" />
    {{ t('shelfSource.shelfFound', { count: String(books) }) }}
  </p>
  <p v-else-if="status === 'error'" class="shelf-source-status error" role="alert">{{ error }}</p>
</template>

<script setup lang="ts">
import Icon from '@/components/Icon.vue';
import { useI18n } from '@/i18n';
import type { SourceCheckStatus } from '@/composables/useSourceCheck';

defineProps<{ status: SourceCheckStatus; books: number; error: string }>();

const { t } = useI18n();
</script>

<style scoped>
.shelf-source-status {
  align-items: center;
  background: #f1f5f9;
  border-radius: 9px;
  color: var(--muted);
  display: flex;
  font-size: 13px;
  gap: 8px;
  margin: 0;
  padding: 9px 12px;
}

.shelf-source-status.ok {
  background: #eaf7f0;
  color: #1a7f4b;
}

.shelf-source-status.error {
  background: #fdecec;
  color: var(--danger);
}

.shelf-source-status-icon {
  flex: none;
  height: 16px;
  width: 16px;
}
</style>
