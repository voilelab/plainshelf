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

    <!-- Reka UI 2.10.5 Tabs defaults unmountOnHide to true. -->
    <TabsRoot v-model="activeSettingsTab" class="settings-tabs" :unmount-on-hide="false">
      <TabsList class="settings-tabs-list" :aria-label="t('settings.title')">
        <TabsTrigger v-if="serverSettingsEditable" value="cover" class="settings-tab-trigger">{{
          t('settings.cover.title')
        }}</TabsTrigger>
        <TabsTrigger value="read-history" class="settings-tab-trigger">{{
          t('settings.readHistory.title')
        }}</TabsTrigger>
        <TabsTrigger value="reader-launch" class="settings-tab-trigger">{{
          t('settings.readerLaunch.title')
        }}</TabsTrigger>
        <TabsTrigger value="language" class="settings-tab-trigger">{{
          t('settings.language.title')
        }}</TabsTrigger>
        <TabsTrigger v-if="serverSettingsEditable" value="nsfw" class="settings-tab-trigger">{{
          t('settings.nsfw.title')
        }}</TabsTrigger>
        <TabsTrigger v-else value="device-nsfw" class="settings-tab-trigger">{{
          t('settings.deviceNsfw.title')
        }}</TabsTrigger>
        <TabsTrigger v-if="serverSettingsEditable" value="import" class="settings-tab-trigger">{{
          t('settings.import.title')
        }}</TabsTrigger>
        <TabsTrigger v-if="serverSettingsEditable" value="logs" class="settings-tab-trigger">{{
          t('settings.logs.title')
        }}</TabsTrigger>
        <TabsTrigger v-if="wasmDemo" value="demo-storage" class="settings-tab-trigger">{{
          t('settings.demoStorage.title')
        }}</TabsTrigger>
        <TabsTrigger value="about" class="settings-tab-trigger">{{ t('settings.about.title') }}</TabsTrigger>
        <TabsTrigger value="shelves" class="settings-tab-trigger">{{ t('settings.shelves.title') }}</TabsTrigger>
      </TabsList>

      <TabsContent v-if="serverSettingsEditable" value="cover" class="settings-tab-content">
        <CoverPanel :value="coverToJpg" :disabled="loading || saving" @change="onCoverToJpgChange" />
      </TabsContent>

      <TabsContent value="read-history" class="settings-tab-content">
        <ReadHistoryPanel
          :value="readHistoryLimit"
          :disabled="loading || saving"
          @change="onReadHistoryLimitChange"
        />
      </TabsContent>

      <TabsContent value="reader-launch" class="settings-tab-content">
        <ReaderLaunchPanel
          :value="readerLaunchMode"
          :disabled="loading || saving"
          @change="onReaderLaunchModeChange"
        />
      </TabsContent>

      <TabsContent value="language" class="settings-tab-content">
        <LanguagePanel />
      </TabsContent>

      <TabsContent v-if="serverSettingsEditable" value="nsfw" class="settings-tab-content">
        <NsfwPanel :value="showNsfw" :disabled="loading || saving" @change="onShowNsfwChange" />
      </TabsContent>

      <TabsContent v-else value="device-nsfw" class="settings-tab-content">
        <DeviceNsfwPanel :value="showNsfwOnDevice" @change="setShowNsfwOnDevice" />
      </TabsContent>

      <TabsContent v-if="serverSettingsEditable" value="import" class="settings-tab-content">
        <EpubImportPanel
          :preset="epubPreset"
          :include-description="epubIncludeDescription"
          :error="epubImportError"
          :disabled="loading || saving"
          :saving="saving"
          @update:preset="onEpubPresetChange"
          @update:include-description="epubIncludeDescription = $event"
          @save="onSaveEpubImportStrategy"
        />
      </TabsContent>

      <TabsContent v-if="serverSettingsEditable" value="logs" class="settings-tab-content">
        <LogRetentionPanel
          :value="logRetentionDays"
          :disabled="loading || saving"
          @change="onLogRetentionDaysChange"
        />
      </TabsContent>

      <TabsContent v-if="wasmDemo" value="demo-storage" class="settings-tab-content">
        <DemoStoragePanel />
      </TabsContent>

      <TabsContent value="about" class="settings-tab-content">
        <AboutPanel />
      </TabsContent>

      <TabsContent value="shelves" class="settings-tab-content">
        <ShelvesPanel />
        <LanSharePanel v-if="lanShareAvailable" />
      </TabsContent>
    </TabsRoot>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui';
import AboutPanel from '@/features/settings/components/AboutPanel.vue';
import CoverPanel from '@/features/settings/components/CoverPanel.vue';
import DemoStoragePanel from '@/features/settings/components/DemoStoragePanel.vue';
import DeviceNsfwPanel from '@/features/settings/components/DeviceNsfwPanel.vue';
import EpubImportPanel from '@/features/settings/components/EpubImportPanel.vue';
import LanSharePanel from '@/features/settings/components/LanSharePanel.vue';
import LanguagePanel from '@/features/settings/components/LanguagePanel.vue';
import LogRetentionPanel from '@/features/settings/components/LogRetentionPanel.vue';
import NsfwPanel from '@/features/settings/components/NsfwPanel.vue';
import ReadHistoryPanel from '@/features/settings/components/ReadHistoryPanel.vue';
import ReaderLaunchPanel from '@/features/settings/components/ReaderLaunchPanel.vue';
import ShelvesPanel from '@/features/settings/components/ShelvesPanel.vue';
import { hasDesktopLANShareBinding } from '@/api/desktop';
import { useDeviceNsfwPreference } from '@/composables/useDeviceNsfwPreference';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useI18n } from '@/i18n';
import { useServerSettingsForm } from '@/features/settings/composables/useServerSettingsForm';
import { useSettingsTabs } from '@/features/settings/composables/useSettingsTabs';
import { isWasmDemo } from '@/features/settings/composables/useDemoStoragePersistence';

const { t } = useI18n();
const { serverSettingsEditable } = useWriteAccess();
const wasmDemo = isWasmDemo();
const lanShareAvailable = hasDesktopLANShareBinding();
const { activeSettingsTab } = useSettingsTabs(serverSettingsEditable, wasmDemo);
const { showNsfw: showNsfwOnDevice, setShowNsfw: setShowNsfwOnDevice } = useDeviceNsfwPreference();

const {
  loading,
  saving,
  error,
  coverToJpg,
  showNsfw,
  logRetentionDays,
  readHistoryLimit,
  readerLaunchMode,
  epubPreset,
  epubIncludeDescription,
  epubImportError,
  loadSettings,
  onCoverToJpgChange,
  onShowNsfwChange,
  onLogRetentionDaysChange,
  onReadHistoryLimitChange,
  onReaderLaunchModeChange,
  onEpubPresetChange,
  onSaveEpubImportStrategy
} = useServerSettingsForm({ serverSettingsEditable });

useDocumentTitle(() => [t('settings.title'), t('app.name')]);

onMounted(() => {
  void loadSettings();
});
</script>

<style scoped src="../styles/settings-form.css"></style>

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

.settings-tabs {
  display: grid;
  gap: 16px;
}

.settings-tabs-list {
  border-bottom: 1px solid var(--border);
  display: flex;
  /* Wraps rather than widening the page when the tabs outgrow it. */
  flex-wrap: wrap;
  gap: 4px;
}

.settings-tab-trigger {
  background: transparent;
  border: 1px solid transparent;
  border-bottom: none;
  border-radius: 8px 8px 0 0;
  color: #475569;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: -1px;
  padding: 8px 14px;
}

.settings-tab-trigger:hover {
  color: #1e293b;
}

.settings-tab-trigger[data-state='active'] {
  background: var(--surface, #fff);
  border-color: var(--border);
  color: #1e293b;
}

.settings-tab-content {
  display: grid;
  gap: 16px;
}

/* Reka keeps every tab panel in the DOM and only marks the inactive ones with
   `hidden`. An author rule outranks the user-agent `[hidden]` default, so the
   rule above would keep those panels as grid items and each one's 16px gap
   would push the active panel further down the further right its tab sits.
   With `unmount-on-hide` off the inactive panels also hold their content, so
   without this rule they would render in full rather than merely take up
   space. */
.settings-tab-content[hidden] {
  display: none;
}
</style>
