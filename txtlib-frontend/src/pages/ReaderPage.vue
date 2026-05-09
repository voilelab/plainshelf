<template>
  <section class="reader-page">
    <div class="reader-shell">
      <header class="reader-toolbar">
        <RouterLink :to="`/books/${id}`" class="reader-back">Back to detail</RouterLink>
        <div class="reader-title">
          <span class="reader-kicker">Reader</span>
          <h2>{{ title || id }}</h2>
        </div>
        <div class="reader-actions">
          <span class="reader-progress">Progress: {{ progress?.percent ?? 0 }}%</span>
          <button class="button reader-bookmark" :disabled="bookmarking" @click="bookmarkCurrent">
            {{ bookmarking ? 'Saving...' : 'Bookmark' }}
          </button>
        </div>
      </header>

      <div v-if="loading" class="loading reader-status">Loading content...</div>
      <div v-else-if="error" class="error reader-status reader-error" role="alert">
        <p>{{ error }}</p>
        <button class="button" type="button" @click="fetchReaderData">Retry</button>
      </div>

      <article v-else class="reader-document">
        <div class="reader-content" @scroll="onScroll" ref="readerRef">
          <div class="reader-text">{{ content }}</div>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useReader } from '../composables/useReader';

const route = useRoute();
const id = computed(() => String(route.params.id));
const { title, content, progress, loading, bookmarking, error, readerRef, fetchReaderData, onScroll, bookmarkCurrent } = useReader(
  () => id.value
);

onMounted(() => {
  void fetchReaderData();
});
</script>

<style scoped>
.reader-page {
  width: 100%;
  min-height: 100vh;
  padding: 28px 18px 56px;
}

.reader-shell {
  max-width: 860px;
  margin: 0 auto;
}

.reader-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 18px;
  margin-bottom: 18px;
  color: #6b5f4a;
}

.reader-back {
  justify-self: start;
  color: #6b5f4a;
  text-decoration: none;
  font-size: 0.95rem;
  letter-spacing: 0.02em;
}

.reader-back:hover {
  color: #4f4434;
}

.reader-title {
  text-align: center;
  min-width: 0;
}

.reader-kicker {
  display: block;
  margin-bottom: 4px;
  font-size: 0.78rem;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: rgba(107, 95, 74, 0.72);
}

.reader-title h2 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: #3f3529;
  overflow-wrap: anywhere;
}

.reader-actions {
  justify-self: end;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  min-width: 0;
}

.reader-progress {
  color: #6b5f4a;
  font-size: 0.92rem;
  white-space: nowrap;
}

.reader-bookmark {
  border-color: rgba(122, 104, 72, 0.18);
  background: rgba(255, 250, 240, 0.86);
  color: #4b3f2f;
  box-shadow: 0 8px 18px rgba(91, 73, 46, 0.08);
}

.reader-bookmark:hover:not(:disabled) {
  background: #fffdf7;
}

.reader-bookmark:disabled {
  cursor: wait;
  opacity: 0.72;
}

.reader-status {
  margin: 0 auto;
  max-width: 760px;
}

.reader-error {
  display: grid;
  gap: 10px;
}

.reader-error p {
  margin: 0;
}

.reader-error .button {
  justify-self: start;
}

.reader-document {
  background: linear-gradient(180deg, rgba(255, 251, 242, 0.98), rgba(253, 248, 237, 0.94));
  border-radius: 18px;
  box-shadow:
    0 16px 40px rgba(96, 75, 44, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.7);
  padding: 22px;
}

.reader-content {
  max-height: 72vh;
  overflow-y: auto;
  padding: 18px 26px;
  scrollbar-gutter: stable both-edges;
}

.reader-text {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  color: #2f2a22;
  font-family: Georgia, 'Times New Roman', 'Noto Serif TC', 'Songti TC', serif;
  font-size: 1.08rem;
  line-height: 1.95;
  letter-spacing: 0.01em;
}

@media (max-width: 720px) {
  .reader-page {
    padding: 18px 12px 36px;
  }

  .reader-toolbar {
    grid-template-columns: 1fr;
    justify-items: start;
    gap: 10px;
    margin-bottom: 14px;
  }

  .reader-title {
    text-align: left;
  }

  .reader-actions {
    justify-self: stretch;
    width: 100%;
    justify-content: space-between;
    flex-wrap: wrap;
  }

  .reader-document {
    border-radius: 14px;
    padding: 12px;
  }

  .reader-content {
    max-height: calc(100vh - 200px);
    padding: 12px 8px 16px;
  }

  .reader-text {
    font-size: 1rem;
    line-height: 1.82;
  }
}
</style>
