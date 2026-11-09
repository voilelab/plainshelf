<template>
  <ConfirmModal
    :open="open"
    :title="t('library.backdrop.title')"
    :cancel-text="t('library.backdrop.done')"
    :close-label="t('library.backdrop.close')"
    confirm-hidden
    @cancel="close"
  >
    <p class="backdrop-hint">{{ t('library.backdrop.hint') }}</p>
    <div class="backdrop-swatches" role="radiogroup" :aria-label="t('library.backdrop.colors')">
      <button
        v-for="swatch in SHELF_BACKDROP_SWATCHES"
        :key="swatch"
        type="button"
        role="radio"
        class="backdrop-swatch"
        :class="`backdrop-swatch-${swatch}`"
        :aria-checked="current?.kind === 'color' && current.swatch === swatch"
        :aria-label="t(`library.backdrop.swatch.${swatch}`)"
        :title="t(`library.backdrop.swatch.${swatch}`)"
        :disabled="busy"
        @click="run(() => setColor(swatch))"
      ></button>
    </div>
    <div class="backdrop-actions">
      <label class="button backdrop-image-button" :class="{ disabled: busy }">
        {{ current?.kind === 'image' ? t('library.backdrop.replaceImage') : t('library.backdrop.chooseImage') }}
        <input
          class="backdrop-file-input"
          type="file"
          accept="image/*"
          :disabled="busy"
          data-testid="backdrop-file"
          @change="onFileChange"
        />
      </label>
      <button type="button" class="button" :disabled="busy || !current" @click="run(clear)">
        {{ t('library.backdrop.reset') }}
      </button>
    </div>
    <p class="backdrop-limit">{{ t('library.backdrop.limit', { max: maxImageMB }) }}</p>
    <p v-if="error" class="backdrop-error" role="alert">{{ error }}</p>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { SHELF_BACKDROP_MAX_IMAGE_BYTES, useShelfBackdrop } from '@/composables/useShelfBackdrop';
import { SHELF_BACKDROP_SWATCHES } from '@/storage/shelfBackdrops';
import { useI18n } from '@/i18n';

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();

const { t } = useI18n();
const { current, setColor, setImage, clear } = useShelfBackdrop();
const busy = ref(false);
const error = ref('');
const maxImageMB = SHELF_BACKDROP_MAX_IMAGE_BYTES / 1024 / 1024;

watch(
  () => props.open,
  (open) => {
    if (open) error.value = '';
  }
);

async function run(action: () => Promise<void>): Promise<void> {
  busy.value = true;
  error.value = '';
  try {
    await action();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    busy.value = false;
  }
}

function onFileChange(event: Event): void {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (file) void run(() => setImage(file));
}

function close(): void {
  if (!busy.value) emit('close');
}
</script>

<style scoped>
.backdrop-hint,
.backdrop-limit {
  color: var(--text-secondary);
  margin: 0 0 12px;
}

.backdrop-limit {
  font-size: 12px;
  margin: 12px 0 0;
}

.backdrop-swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 16px;
}

.backdrop-swatch {
  border: 1px solid var(--border-strong);
  border-radius: 999px;
  cursor: pointer;
  height: 32px;
  padding: 0;
  width: 32px;
}

.backdrop-swatch[aria-checked='true'] {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.backdrop-swatch:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.backdrop-swatch-sage {
  background: var(--shelf-backdrop-sage);
}

.backdrop-swatch-sky {
  background: var(--shelf-backdrop-sky);
}

.backdrop-swatch-lavender {
  background: var(--shelf-backdrop-lavender);
}

.backdrop-swatch-rose {
  background: var(--shelf-backdrop-rose);
}

.backdrop-swatch-sand {
  background: var(--shelf-backdrop-sand);
}

.backdrop-swatch-slate {
  background: var(--shelf-backdrop-slate);
}

.backdrop-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.backdrop-image-button {
  position: relative;
}

.backdrop-image-button.disabled {
  opacity: 0.6;
  pointer-events: none;
}

.backdrop-file-input {
  cursor: pointer;
  inset: 0;
  opacity: 0;
  position: absolute;
  width: 100%;
}

.backdrop-error {
  background: var(--danger-soft);
  border: 1px solid var(--danger-border);
  border-radius: 8px;
  color: var(--danger-ink);
  margin: 12px 0 0;
  padding: 10px;
}
</style>
