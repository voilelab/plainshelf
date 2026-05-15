<template>
  <Teleport to="body">
    <div v-if="open" class="confirm-modal-overlay" role="presentation" @click="onBackdropClick">
      <section
        class="panel confirm-modal"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        :aria-describedby="descriptionId"
        @click.stop
      >
        <header class="confirm-modal-header">
          <h2 :id="titleId">{{ title }}</h2>
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
            :disabled="busy"
            @click="emit('confirm')"
          >
            {{ busy ? busyText : confirmText }}
          </button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    message?: string;
    confirmText?: string;
    cancelText?: string;
    busyText?: string;
    busy?: boolean;
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
    closeOnBackdrop: true,
    closeLabel: 'Close confirmation dialog',
    variant: 'primary'
  }
);

const emit = defineEmits<{
  cancel: [];
  confirm: [];
}>();

const titleId = `confirm-modal-title-${Math.random().toString(36).slice(2)}`;
const descriptionId = `confirm-modal-description-${Math.random().toString(36).slice(2)}`;
const confirmButton = ref<HTMLButtonElement | null>(null);
const confirmVariant = computed(() => ({
  primary: props.variant === 'primary',
  danger: props.variant === 'danger'
}));

function onBackdropClick(): void {
  if (props.closeOnBackdrop && !props.busy) {
    emit('cancel');
  }
}

function onDocumentKeydown(event: KeyboardEvent): void {
  if (!props.open || props.busy) {
    return;
  }

  if (event.key === 'Escape') {
    emit('cancel');
  }
}

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      return;
    }

    await nextTick();
    confirmButton.value?.focus();
  }
);

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
});
</script>

<style scoped>
.confirm-modal-overlay {
  align-items: center;
  background: rgba(15, 23, 42, 0.42);
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 16px;
  position: fixed;
  z-index: 80;
}

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
