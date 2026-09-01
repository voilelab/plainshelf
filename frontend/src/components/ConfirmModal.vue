<template>
  <component
    :is="shell"
    v-bind="shellProps"
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
        <!-- On the destructive path the cancel button must be the element
             AlertDialogCancel registers, because AlertDialogContent focuses
             exactly that element when the dialog opens. Closing then travels
             the same route as ESC — AlertDialogRoot's update:open — so this
             button deliberately has no click handler of its own. -->
        <AlertDialogCancel v-if="destructive" as-child>
          <button class="button" type="button" :disabled="busy">
            {{ cancelText }}
          </button>
        </AlertDialogCancel>
        <button v-else class="button" type="button" :disabled="busy" @click="emit('cancel')">
          {{ cancelText }}
        </button>
        <!-- Not AlertDialogAction: that closes the dialog on click, and several
             callers keep it open after confirming to show progress (see the
             empty-trash flow in TrashPage.vue). The parent owns the close. -->
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
  }>(),
  {
    message: '',
    busy: false,
    confirmDisabled: false,
    closeOnBackdrop: true,
    variant: 'primary'
  }
);

// The four text props fall back through computeds rather than through
// withDefaults: a default is evaluated once where the component is defined, so
// a t() call there would freeze the English string in place and never follow a
// locale change. Same pattern as DeleteModal.vue.
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
const confirmVariant = computed(() => ({
  primary: props.variant === 'primary',
  danger: props.variant === 'danger'
}));

// The danger variant is the irreversible one, so it gets alert-dialog
// semantics: role="alertdialog", no backdrop dismissal, focus on cancel.
// Everything else — editing, importing, choosing a font — stays an ordinary
// dialog on BaseDialog.
const destructive = computed(() => props.variant === 'danger');
const shell = computed(() => (destructive.value ? BaseAlertDialog : BaseDialog));
const shellProps = computed(() => ({
  open: props.open,
  title: props.title,
  busy: props.busy,
  describedBy: descriptionId,
  // BaseAlertDialog has no such prop: an alert dialog is never dismissed by
  // its backdrop, whatever the caller asks for.
  ...(destructive.value ? {} : { dismissible: props.closeOnBackdrop })
}));

watch(
  () => props.open,
  async (open) => {
    // Destructive dialogs must open on cancel, and AlertDialogContent already
    // puts focus there; stealing it back to confirm would defeat the point.
    if (!open || destructive.value) {
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
