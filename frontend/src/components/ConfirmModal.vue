<template>
  <component
    :is="shell"
    v-bind="shellProps"
    @close="emit('cancel')"
  >
    <section ref="panel" class="panel confirm-modal">
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
        <!-- Reka UI 2.10.5 AlertDialogContent focuses its registered Cancel element on open. -->
        <AlertDialogCancel v-if="destructive" as-child>
          <button class="button" type="button" :disabled="busy">
            {{ cancelText }}
          </button>
        </AlertDialogCancel>
        <button v-else class="button" type="button" :disabled="busy" @click="emit('cancel')">
          {{ cancelText }}
        </button>
        <!-- Reka UI 2.10.5 AlertDialogAction wraps DialogClose and closes on click. -->
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
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue';
import { AlertDialogCancel } from 'reka-ui';
import BaseAlertDialog from './BaseAlertDialog.vue';
import BaseDialog from './BaseDialog.vue';
import { useI18n } from '@/i18n';

const { t } = useI18n();

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
    initialFocus?: string;
  }>(),
  {
    message: '',
    busy: false,
    confirmDisabled: false,
    closeOnBackdrop: true,
    variant: 'primary',
    initialFocus: undefined
  }
);

const confirmText = computed(() => props.confirmText ?? t('common.confirm'));
const cancelText = computed(() => props.cancelText ?? t('common.cancel'));
const busyText = computed(() => props.busyText ?? t('common.working'));
const closeLabel = computed(() => props.closeLabel ?? t('common.closeDialog'));

const emit = defineEmits<{
  cancel: [];
  confirm: [];
}>();

const descriptionId = `confirm-modal-description-${useId()}`;
const confirmButton = ref<HTMLButtonElement | null>(null);
const panel = ref<HTMLElement | null>(null);
const confirmVariant = computed(() => ({
  primary: props.variant === 'primary',
  danger: props.variant === 'danger'
}));

const destructive = computed(() => props.variant === 'danger');
const shell = computed(() => (destructive.value ? BaseAlertDialog : BaseDialog));
const shellProps = computed(() => ({
  open: props.open,
  title: props.title,
  busy: props.busy,
  describedBy: descriptionId,
  ...(destructive.value ? {} : { dismissible: props.closeOnBackdrop })
}));

watch(
  () => props.open,
  async (open) => {
    if (!open || destructive.value) {
      return;
    }

    await nextTick();

    if (props.initialFocus) {
      const target = panel.value?.querySelector<HTMLElement>(props.initialFocus);
      if (target) {
        target.focus();
        return;
      }
    }

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
  /* Callers put unbroken strings here — a dropped file name, a shelf path. The
     panel is a grid, so such a string widens the column track past the panel's
     own 440px and pushes the header's close button outside the painted card.
     `anywhere` rather than `break-word`: only the former shrinks the
     min-content width the track is sized from. */
  overflow-wrap: anywhere;
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
