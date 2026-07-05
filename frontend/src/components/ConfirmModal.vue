<template>
  <BaseDialog
    :open="open"
    :title="title"
    :dismissible="closeOnBackdrop"
    :busy="busy"
    :described-by="descriptionId"
    @close="emit('cancel')"
  >
    <section class="panel confirm-modal">
      <header class="confirm-modal-header">
        <h2>{{ title }}</h2>
        <button
          class="confirm-modal-close"
          type="button"
          :aria-label="closeLabel"
          :disabled="busy"
          @click="emit('cancel')"
        >
          ×
        </button>
      </header>

      <div :id="descriptionId" class="confirm-modal-body">
        <slot>
          <p>{{ message }}</p>
        </slot>
      </div>

      <footer class="confirm-modal-actions">
        <button class="button" type="button" :disabled="busy" @click="emit('cancel')">
          {{ cancelText }}
        </button>
        <button
          ref="confirmButton"
          class="button"
          :class="confirmVariant"
          type="button"
          :disabled="busy || confirmDisabled"
          @click="emit('confirm')"
        >
          {{ busy ? busyText : confirmText }}
        </button>
      </footer>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue';
import BaseDialog from './BaseDialog.vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    message?: string;
    confirmText?: string;
    cancelText?: string;
    busyText?: string;
    busy?: boolean;
    confirmDisabled?: boolean;
    closeOnBackdrop?: boolean;
    closeLabel?: string;
    variant?: 'primary' | 'danger';
  }>(),
  {
    message: '',
    confirmText: 'Confirm',
    cancelText: 'Cancel',
    busyText: 'Working...',
    busy: false,
    confirmDisabled: false,
    closeOnBackdrop: true,
    closeLabel: 'Close confirmation dialog',
    variant: 'primary'
  }
);

const emit = defineEmits<{
  cancel: [];
  confirm: [];
}>();

const descriptionId = `confirm-modal-description-${useId()}`;
const confirmButton = ref<HTMLButtonElement | null>(null);
const confirmVariant = computed(() => ({
  primary: props.variant === 'primary',
  danger: props.variant === 'danger'
}));

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      return;
    }

    await nextTick();
    if (confirmButton.value && !confirmButton.value.disabled) {
      confirmButton.value.focus();
    }
  }
);
</script>

<style scoped>
.confirm-modal {
  display: grid;
  gap: 14px;
  max-width: 440px;
  padding: 18px;
  width: min(100%, 440px);
}

.confirm-modal-header {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.confirm-modal-header h2 {
  font-size: 20px;
  line-height: 1.2;
  margin: 0;
}

.confirm-modal-close {
  align-items: center;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  cursor: pointer;
  display: inline-flex;
  font-size: 20px;
  height: 32px;
  justify-content: center;
  line-height: 1;
  width: 32px;
}

.confirm-modal-close:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.confirm-modal-body {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.5;
}

.confirm-modal-body :deep(p) {
  margin: 0;
}

.confirm-modal-body :deep(p + p) {
  margin-top: 8px;
}

.confirm-modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.button.danger:hover:not(:disabled) {
  opacity: 0.85;
}

@media (max-width: 520px) {
  .confirm-modal {
    max-width: 100%;
    padding: 16px;
  }

  .confirm-modal-actions {
    flex-direction: column-reverse;
  }

  .confirm-modal-actions .button {
    width: 100%;
  }
}
</style>
