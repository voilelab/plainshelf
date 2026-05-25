<template>
  <div v-if="open" class="modal-overlay" role="presentation" @click="onBackdropClick">
    <section
      class="panel cover-gen-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="cover-gen-title"
      @click.stop
    >
      <header class="modal-header">
        <h2 id="cover-gen-title">Generate Cover</h2>
        <button
          class="icon-close"
          type="button"
          aria-label="Close dialog"
          :disabled="saving"
          @click="onClose"
        >
          ×
        </button>
      </header>

      <div class="modal-body">
        <div class="preview-col">
          <canvas
            ref="canvasRef"
            class="cover-canvas"
            :width="CANVAS_W"
            :height="CANVAS_H"
          />
        </div>

        <div class="controls-col">
          <label class="field">
            <span class="field-label">Title</span>
            <input v-model="titleText" class="input" type="text" :disabled="saving" />
          </label>

          <label class="field">
            <span class="field-label">Author</span>
            <input v-model="authorText" class="input" type="text" placeholder="(no author)" :disabled="saving" />
          </label>

          <label class="field">
            <span class="field-label">Background style</span>
            <select v-model="bgStyle" class="input" :disabled="saving">
              <option v-for="opt in bgStyleOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </label>

          <label class="field">
            <span class="field-label">Layout</span>
            <select v-model="layout" class="input" :disabled="saving">
              <option v-for="opt in layoutOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </label>

          <div v-if="saveError" class="error-msg" role="alert">{{ saveError }}</div>
        </div>
      </div>

      <div class="modal-actions">
        <button class="button" type="button" :disabled="saving" @click="onClose">Cancel</button>
        <button class="button primary" type="button" :disabled="saving" @click="onSave">
          {{ saving ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { uploadBookCoverBlob } from '../api/books';

const CANVAS_W = 400;
const CANVAS_H = 600;
const JPEG_EXPORT_QUALITY = 0.92;

// Layout spacing constants
const LAYOUT_PADDING = 36;
const AUTHOR_LINE_HEIGHT = 28;

type BgStyle = 'plain-light' | 'plain-dark' | 'warm-paper' | 'soft-gradient' | 'minimal-solid';
type Layout = 'centered' | 'top-bottom' | 'large-title' | 'minimal';

interface BgStyleOption {
  value: BgStyle;
  label: string;
}

interface LayoutOption {
  value: Layout;
  label: string;
}

const bgStyleOptions: BgStyleOption[] = [
  { value: 'plain-light', label: 'Plain light' },
  { value: 'plain-dark', label: 'Plain dark' },
  { value: 'warm-paper', label: 'Warm paper' },
  { value: 'soft-gradient', label: 'Soft gradient' },
  { value: 'minimal-solid', label: 'Minimal solid color' }
];

const layoutOptions: LayoutOption[] = [
  { value: 'centered', label: 'Centered title, author below' },
  { value: 'top-bottom', label: 'Title near top, author near bottom' },
  { value: 'large-title', label: 'Large title centered' },
  { value: 'minimal', label: 'Minimal layout' }
];

const props = defineProps<{
  open: boolean;
  bookId: string;
  initialTitle: string;
  initialAuthor: string;
}>();

const emit = defineEmits<{
  close: [];
  saved: [];
}>();

const canvasRef = ref<HTMLCanvasElement | null>(null);
const titleText = ref('');
const authorText = ref('');
const bgStyle = ref<BgStyle>('plain-light');
const layout = ref<Layout>('centered');
const saving = ref(false);
const saveError = ref('');

// ─── Background painters ────────────────────────────────────────────────────

interface BgConfig {
  textColor: string;
  mutedColor: string;
  paint: (ctx: CanvasRenderingContext2D) => void;
}

function getBgConfig(ctx: CanvasRenderingContext2D, style: BgStyle): BgConfig {
  const w = CANVAS_W;
  const h = CANVAS_H;

  switch (style) {
    case 'plain-dark':
      return {
        textColor: '#f0f0f0',
        mutedColor: '#b0b8c8',
        paint(c) {
          c.fillStyle = '#1a1a2e';
          c.fillRect(0, 0, w, h);
        }
      };
    case 'warm-paper':
      return {
        textColor: '#3d2b1f',
        mutedColor: '#7a5c44',
        paint(c) {
          c.fillStyle = '#f5f0e8';
          c.fillRect(0, 0, w, h);
          // subtle texture lines
          c.strokeStyle = 'rgba(120,90,60,0.06)';
          c.lineWidth = 1;
          for (let y = 20; y < h; y += 28) {
            c.beginPath();
            c.moveTo(0, y);
            c.lineTo(w, y);
            c.stroke();
          }
        }
      };
    case 'soft-gradient':
      return {
        textColor: '#ffffff',
        mutedColor: '#d8dff0',
        paint(c) {
          const grad = c.createLinearGradient(0, 0, w, h);
          grad.addColorStop(0, '#667eea');
          grad.addColorStop(1, '#764ba2');
          c.fillStyle = grad;
          c.fillRect(0, 0, w, h);
        }
      };
    case 'minimal-solid':
      return {
        textColor: '#ffffff',
        mutedColor: '#c8deff',
        paint(c) {
          c.fillStyle = '#1f6feb';
          c.fillRect(0, 0, w, h);
        }
      };
    case 'plain-light':
    default:
      return {
        textColor: '#1a1a2e',
        mutedColor: '#66758a',
        paint(c) {
          c.fillStyle = '#ffffff';
          c.fillRect(0, 0, w, h);
          // subtle top/bottom accent bars
          c.fillStyle = '#e8eef5';
          c.fillRect(0, 0, w, 6);
          c.fillRect(0, h - 6, w, 6);
        }
      };
  }
}

// ─── Text helpers ────────────────────────────────────────────────────────────

function wrapText(
  ctx: CanvasRenderingContext2D,
  text: string,
  maxWidth: number
): string[] {
  if (!text) return [];
  const words = text.split(' ');
  const lines: string[] = [];
  let current = '';

  for (const word of words) {
    const test = current ? `${current} ${word}` : word;
    if (ctx.measureText(test).width <= maxWidth) {
      current = test;
    } else {
      if (current) lines.push(current);
      // If a single word is wider than maxWidth, push it as-is
      current = word;
    }
  }
  if (current) lines.push(current);
  return lines;
}

function drawTextBlock(
  ctx: CanvasRenderingContext2D,
  lines: string[],
  centerX: number,
  startY: number,
  lineHeight: number
): void {
  for (let i = 0; i < lines.length; i++) {
    ctx.fillText(lines[i], centerX, startY + i * lineHeight);
  }
}

// ─── Main render ─────────────────────────────────────────────────────────────

function renderCover(): void {
  const canvas = canvasRef.value;
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  const w = CANVAS_W;
  const h = CANVAS_H;
  const title = titleText.value.trim() || '(no title)';
  const author = authorText.value.trim();
  const cfg = getBgConfig(ctx, bgStyle.value);

  // Paint background
  cfg.paint(ctx);

  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';

  const pad = LAYOUT_PADDING;
  const maxW = w - pad * 2;

  switch (layout.value) {
    case 'top-bottom': {
      // Title near top
      ctx.font = `bold 32px 'Segoe UI', 'Avenir Next', sans-serif`;
      ctx.fillStyle = cfg.textColor;
      const titleLines = wrapText(ctx, title, maxW);
      const titleLineH = 42;
      const titleBlock = titleLines.length * titleLineH;
      const titleY = 80;
      drawTextBlock(ctx, titleLines, w / 2, titleY, titleLineH);

      // Separator
      if (titleLines.length > 0) {
        const sepY = titleY + titleBlock + 18;
        ctx.strokeStyle = cfg.mutedColor;
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.moveTo(w / 2 - 40, sepY);
        ctx.lineTo(w / 2 + 40, sepY);
        ctx.stroke();
      }

      // Author near bottom
      if (author) {
        ctx.font = `18px 'Segoe UI', 'Avenir Next', sans-serif`;
        ctx.fillStyle = cfg.mutedColor;
        const authorLines = wrapText(ctx, author, maxW);
        const authorLineH = 26;
        const authorBlock = authorLines.length * authorLineH;
        const authorY = h - pad - authorBlock;
        drawTextBlock(ctx, authorLines, w / 2, authorY, authorLineH);
      }
      break;
    }

    case 'large-title': {
      // Large title centered, author small below
      ctx.font = `bold 44px 'Segoe UI', 'Avenir Next', sans-serif`;
      ctx.fillStyle = cfg.textColor;
      const titleLines = wrapText(ctx, title, maxW);
      const titleLineH = 56;
      const titleBlock = titleLines.length * titleLineH;
      let authorLines: string[] = [];
      let authorBlock = 0;
      if (author) {
        ctx.font = `16px 'Segoe UI', 'Avenir Next', sans-serif`;
        authorLines = wrapText(ctx, author, maxW);
        authorBlock = authorLines.length * AUTHOR_LINE_HEIGHT;
      }
      const totalBlock = titleBlock + (authorLines.length > 0 ? 20 + authorBlock : 0);
      const startY = (h - totalBlock) / 2;

      ctx.font = `bold 44px 'Segoe UI', 'Avenir Next', sans-serif`;
      ctx.fillStyle = cfg.textColor;
      drawTextBlock(ctx, titleLines, w / 2, startY, titleLineH);

      if (authorLines.length > 0) {
        ctx.font = `16px 'Segoe UI', 'Avenir Next', sans-serif`;
        ctx.fillStyle = cfg.mutedColor;
        drawTextBlock(ctx, authorLines, w / 2, startY + titleBlock + 20, AUTHOR_LINE_HEIGHT);
      }
      break;
    }

    case 'minimal': {
      // Bottom-left aligned, small text
      ctx.textAlign = 'left';
      const leftPad = 40;
      const botPad = 60;

      if (author) {
        ctx.font = `14px 'Segoe UI', 'Avenir Next', sans-serif`;
        ctx.fillStyle = cfg.mutedColor;
        ctx.fillText(author, leftPad, h - botPad);
      }

      ctx.font = `bold 28px 'Segoe UI', 'Avenir Next', sans-serif`;
      ctx.fillStyle = cfg.textColor;
      const titleLines = wrapText(ctx, title, w - leftPad * 2);
      const titleLineH = 38;
      const titleBlock = titleLines.length * titleLineH;
      const titleY = h - botPad - (author ? 28 : 0) - titleBlock - 8;
      drawTextBlock(ctx, titleLines, leftPad, titleY, titleLineH);

      // Top accent line
      ctx.strokeStyle = cfg.mutedColor;
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(leftPad, titleY - 16);
      ctx.lineTo(leftPad + 48, titleY - 16);
      ctx.stroke();

      ctx.textAlign = 'center';
      break;
    }

    case 'centered':
    default: {
      // Centered title, author below
      ctx.font = `bold 34px 'Segoe UI', 'Avenir Next', sans-serif`;
      ctx.fillStyle = cfg.textColor;
      const titleLines = wrapText(ctx, title, maxW);
      const titleLineH = 46;
      const titleBlock = titleLines.length * titleLineH;

      let authorBlock = 0;
      if (author) {
        ctx.font = `18px 'Segoe UI', 'Avenir Next', sans-serif`;
        const authorLines = wrapText(ctx, author, maxW);
        authorBlock = authorLines.length * AUTHOR_LINE_HEIGHT;
      }

      const gap = author ? AUTHOR_LINE_HEIGHT : 0;
      const totalBlock = titleBlock + gap + authorBlock;
      const startY = (h - totalBlock) / 2;

      ctx.font = `bold 34px 'Segoe UI', 'Avenir Next', sans-serif`;
      ctx.fillStyle = cfg.textColor;
      drawTextBlock(ctx, titleLines, w / 2, startY, titleLineH);

      if (author) {
        ctx.font = `18px 'Segoe UI', 'Avenir Next', sans-serif`;
        ctx.fillStyle = cfg.mutedColor;
        const authorLines = wrapText(ctx, author, maxW);
        drawTextBlock(ctx, authorLines, w / 2, startY + titleBlock + gap, AUTHOR_LINE_HEIGHT);
      }
      break;
    }
  }
}

// ─── Watchers ────────────────────────────────────────────────────────────────

watch([titleText, authorText, bgStyle, layout], () => {
  void nextTick(renderCover);
});

watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    titleText.value = props.initialTitle;
    authorText.value = props.initialAuthor;
    bgStyle.value = 'plain-light';
    layout.value = 'centered';
    saveError.value = '';
    saving.value = false;
    await nextTick();
    renderCover();
  }
);

// ─── Actions ─────────────────────────────────────────────────────────────────

function onClose(): void {
  if (saving.value) return;
  emit('close');
}

function onBackdropClick(): void {
  onClose();
}

async function onSave(): Promise<void> {
  if (saving.value) return;
  const canvas = canvasRef.value;
  if (!canvas) {
    saveError.value = 'Cover generation failed: canvas not available.';
    return;
  }

  saving.value = true;
  saveError.value = '';

  try {
    // Render one final time to ensure latest state
    renderCover();

    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob(resolve, 'image/jpeg', JPEG_EXPORT_QUALITY);
    });

    if (!blob) {
      throw new Error('Failed to export cover image.');
    }

    await uploadBookCoverBlob(props.bookId, blob);
    emit('saved');
    emit('close');
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : 'Failed to save cover.';
  } finally {
    saving.value = false;
  }
}

// ─── Keyboard ────────────────────────────────────────────────────────────────

function onDocumentKeydown(event: KeyboardEvent): void {
  if (!props.open || saving.value) return;
  if (event.key === 'Escape') {
    emit('close');
  }
}

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
});
</script>

<style scoped>
.modal-overlay {
  align-items: center;
  background: rgba(15, 23, 42, 0.38);
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 16px;
  position: fixed;
  z-index: 50;
}

.cover-gen-modal {
  display: grid;
  gap: 16px;
  max-height: calc(100vh - 32px);
  overflow: auto;
  padding: 20px;
  width: min(100%, 760px);
}

.modal-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.modal-header h2 {
  margin: 0;
}

.icon-close {
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

.icon-close:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.modal-body {
  display: grid;
  gap: 20px;
  grid-template-columns: auto 1fr;
  align-items: start;
}

.preview-col {
  display: flex;
  justify-content: center;
}

.cover-canvas {
  border: 1px solid var(--border);
  border-radius: 8px;
  display: block;
  width: 200px;
  height: 300px;
}

.controls-col {
  display: grid;
  gap: 12px;
}

.field {
  display: grid;
  gap: 5px;
}

.field-label {
  color: var(--muted);
  font-size: 13px;
}

.error-msg {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  font-size: 13px;
  padding: 8px 12px;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

@media (max-width: 600px) {
  .modal-body {
    grid-template-columns: 1fr;
  }

  .preview-col {
    order: -1;
  }

  .cover-canvas {
    width: 160px;
    height: 240px;
  }

  .cover-gen-modal {
    padding: 14px;
  }
}
</style>
