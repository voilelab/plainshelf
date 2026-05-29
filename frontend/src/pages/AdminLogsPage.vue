<template>
  <section class="admin-logs-page">
    <header class="page-header">
      <div>
        <h2>{{ t('adminLogs.title') }}</h2>
        <p>{{ t('adminLogs.description') }}</p>
      </div>
      <button class="button" type="button" :disabled="loadingLogs" @click="loadLogs">
        {{ t('common.retry') }}
      </button>
    </header>

    <div class="filters">
      <label class="field">
        <span>{{ t('adminLogs.name') }}</span>
        <select v-model="selectedName" :disabled="loadingLogs || nameOptions.length === 0">
          <option v-for="option in nameOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </label>

      <label class="field">
        <span>{{ t('adminLogs.date') }}</span>
        <input
          v-model="selectedDate"
          type="date"
          :min="dateInputMin"
          :max="dateInputMax"
          :disabled="loadingLogs || dateOptions.length === 0"
        />
      </label>
    </div>

    <p v-if="error" class="message error" role="alert">
      {{ error }}
    </p>
    <p v-else-if="loadingLogs" class="message">
      {{ t('adminLogs.loadingList') }}
    </p>
    <p v-else-if="logs.length === 0" class="message">
      {{ t('adminLogs.empty') }}
    </p>

    <template v-else>
      <dl v-if="selectedLog" class="log-meta">
        <div>
          <dt>{{ t('adminLogs.filename') }}</dt>
          <dd>{{ selectedLog.filename }}</dd>
        </div>
        <div>
          <dt>{{ t('adminLogs.source') }}</dt>
          <dd>{{ selectedLog.source || '—' }}</dd>
        </div>
      </dl>

      <p v-if="loadingContent" class="message">
        {{ t('adminLogs.loadingContent') }}
      </p>
      <pre v-else class="log-content">{{ content || t('adminLogs.emptyContent') }}</pre>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { getLogContent, listLogs, type LogFileEntry } from '../api/logs';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useI18n } from '../i18n';

interface LogNameOption {
  value: string;
  label: string;
}

const { t } = useI18n();

const logs = ref<LogFileEntry[]>([]);
const loadingLogs = ref(false);
const loadingContent = ref(false);
const error = ref('');
const content = ref('');
const selectedName = ref('');
const selectedDate = ref('');
const activeLogRequest = ref(0);

useDocumentTitle(() => [t('adminLogs.title'), t('app.name')]);

function getLogName(entry: LogFileEntry): string {
  return entry.source || entry.filename;
}

const nameOptions = computed<LogNameOption[]>(() => {
  const seen = new Set<string>();
  const options: LogNameOption[] = [];

  for (const entry of logs.value) {
    const value = getLogName(entry);
    if (seen.has(value)) {
      continue;
    }
    seen.add(value);
    options.push({
      value,
      label: value
    });
  }

  return options;
});

const dateOptions = computed<string[]>(() => {
  if (!selectedName.value) {
    return [];
  }

  const seen = new Set<string>();
  const options: string[] = [];

  for (const entry of logs.value) {
    if (getLogName(entry) !== selectedName.value || seen.has(entry.date)) {
      continue;
    }
    seen.add(entry.date);
    options.push(entry.date);
  }

  return options;
});

const dateInputMin = computed(() => [...dateOptions.value].sort()[0] ?? '');

const dateInputMax = computed(() => {
  const sortedDates = [...dateOptions.value].sort();
  return sortedDates[sortedDates.length - 1] ?? '';
});

const selectedLog = computed(() =>
  logs.value.find((entry) => getLogName(entry) === selectedName.value && entry.date === selectedDate.value)
);

function syncSelection(): void {
  if (nameOptions.value.length === 0) {
    selectedName.value = '';
    selectedDate.value = '';
    return;
  }

  if (!nameOptions.value.some((option) => option.value === selectedName.value)) {
    selectedName.value = nameOptions.value[0].value;
  }

  if (!dateOptions.value.includes(selectedDate.value)) {
    selectedDate.value = dateOptions.value[0] ?? '';
  }
}

async function loadLogs(): Promise<void> {
  loadingLogs.value = true;
  error.value = '';

  try {
    logs.value = await listLogs();
    syncSelection();
  } catch (err) {
    logs.value = [];
    content.value = '';
    error.value = err instanceof Error ? err.message : t('adminLogs.loadFailed');
  } finally {
    loadingLogs.value = false;
  }
}

async function loadContent(log: LogFileEntry): Promise<void> {
  const requestId = ++activeLogRequest.value;
  loadingContent.value = true;
  error.value = '';

  try {
    const nextContent = await getLogContent(log.id);
    if (requestId !== activeLogRequest.value) {
      return;
    }
    content.value = nextContent;
  } catch (err) {
    if (requestId !== activeLogRequest.value) {
      return;
    }
    content.value = '';
    error.value = err instanceof Error ? err.message : t('adminLogs.loadContentFailed');
  } finally {
    if (requestId === activeLogRequest.value) {
      loadingContent.value = false;
    }
  }
}

watch([nameOptions, dateOptions], syncSelection, { immediate: true });

watch(
  selectedLog,
  (log) => {
    if (!log) {
      activeLogRequest.value += 1;
      loadingContent.value = false;
      content.value = '';
      return;
    }

    void loadContent(log);
  },
  { immediate: true }
);

onMounted(() => {
  void loadLogs();
});
</script>

<style scoped>
.admin-logs-page {
  display: grid;
  gap: 16px;
}

.page-header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.page-header h2 {
  margin: 0;
}

.page-header p {
  color: #475569;
  margin: 6px 0 0;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.field {
  display: grid;
  gap: 6px;
  min-width: 220px;
}

.field span,
.log-meta dt {
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.field select,
.field input {
  background: #ffffff;
  border: 1px solid var(--border);
  border-radius: 8px;
  font: inherit;
  min-height: 38px;
  padding: 8px 10px;
}

.message {
  margin: 0;
}

.error {
  color: #b91c1c;
}

.log-meta {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  margin: 0;
}

.log-meta div {
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 12px;
}

.log-meta dd {
  margin: 6px 0 0;
  word-break: break-word;
}

.log-content {
  background: #0f172a;
  border-radius: 12px;
  color: #e2e8f0;
  font-family: ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace;
  font-size: 13px;
  line-height: 1.5;
  margin: 0;
  max-height: calc(100vh - 280px);
  min-height: 320px;
  overflow: auto;
  padding: 16px;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
