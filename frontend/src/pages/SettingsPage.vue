<template>
  <section class="settings-page">
    <header class="settings-header">
      <div>
        <h2>{{ t('settings.title') }}</h2>
        <p>{{ t('settings.description') }}</p>
      </div>
      <button class="button" type="button" :disabled="loading || saving" @click="loadSettings">
        {{ t('common.retry') }}
      </button>
    </header>

    <p v-if="error" class="settings-message settings-message-error" role="alert">
      {{ error }}
    </p>

    <section class="panel settings-group">
      <h3>{{ t('settings.cover.title') }}</h3>
      <label class="setting-item">
        <div>
          <div class="setting-label">{{ t('settings.coverToJpg.label') }}</div>
          <p class="setting-description">{{ t('settings.coverToJpg.description') }}</p>
        </div>
        <input
          class="setting-checkbox"
          type="checkbox"
          :checked="coverToJpg"
          :disabled="loading || saving"
          @change="onCoverToJpgChange"
        />
      </label>
    </section>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { getCoverToJpgSetting, setCoverToJpgSetting } from '../api/settings';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useI18n } from '../i18n';

const { t } = useI18n();
const loading = ref(false);
const saving = ref(false);
const error = ref('');
const coverToJpg = ref(false);

useDocumentTitle(() => [t('settings.title'), t('app.name')]);

async function loadSettings(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    coverToJpg.value = await getCoverToJpgSetting();
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('settings.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function onCoverToJpgChange(event: Event): Promise<void> {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }

  const nextValue = target.checked;
  const prevValue = coverToJpg.value;
  coverToJpg.value = nextValue;
  saving.value = true;
  error.value = '';

  try {
    await setCoverToJpgSetting(nextValue);
  } catch (err) {
    coverToJpg.value = prevValue;
    target.checked = prevValue;
    error.value = err instanceof Error ? err.message : t('settings.saveFailed');
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void loadSettings();
});
</script>

<style scoped>
.settings-page {
  display: grid;
  gap: 16px;
  max-width: 760px;
}

.settings-header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.settings-header h2 {
  margin: 0;
}

.settings-header p {
  color: #475569;
  margin: 6px 0 0;
}

.settings-message {
  margin: 0;
}

.settings-message-error {
  color: #b91c1c;
}

.settings-group {
  display: grid;
  gap: 12px;
  padding: 16px;
}

.settings-group h3 {
  margin: 0;
}

.setting-item {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.setting-label {
  font-weight: 600;
}

.setting-description {
  color: #475569;
  font-size: 13px;
  margin: 4px 0 0;
}

.setting-checkbox {
  cursor: pointer;
  height: 18px;
  width: 18px;
}

.setting-checkbox:disabled {
  cursor: not-allowed;
}
</style>
