<template>
  <FontLicenseModal
    :open="selectedFontLicense !== null"
    :title="t('settings.about.licenseTitle', { font: selectedFontLicense?.name ?? '' })"
    :text="fontLicenseText"
    :loading="fontLicenseLoading"
    :error="fontLicenseError"
    :loading-label="t('settings.about.licenseLoading')"
    :close-label="t('settings.about.licenseClose')"
    @close="closeFontLicense"
  />

  <section class="panel settings-group">
    <h3>{{ t('settings.about.title') }}</h3>
    <p class="setting-description">{{ t('settings.about.description') }}</p>
    <div class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.about.version') }}</div>
      </div>
      <span class="setting-value">{{ version || '—' }}</span>
    </div>
    <div class="setting-item">
      <div>
        <div class="setting-label">{{ t('settings.about.repository') }}</div>
      </div>
      <a class="setting-link" :href="githubRepoUrl" target="_blank" rel="noreferrer" @click="onRepositoryLinkClick">
        {{ githubRepoUrl }}
      </a>
    </div>
    <div class="font-license-section">
      <div class="setting-label">{{ t('settings.about.thirdPartyFonts') }}</div>
      <article v-for="font in bundledFonts" :key="font.name" class="font-license-item">
        <strong>{{ font.name }}</strong>
        <span class="setting-description">{{ t('settings.about.fontAttribution') }}</span>
        <span class="font-license-links">
          <a
            class="setting-link"
            :href="font.source"
            target="_blank"
            rel="noreferrer"
            @click="onBundledFontSourceClick($event, font.source)"
          >
            {{ t('settings.about.source') }}
          </a>
          <a class="setting-link" :href="font.license" @click.prevent="openFontLicense(font)">
            {{ t('settings.about.license') }}
          </a>
        </span>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import FontLicenseModal from '@/features/settings/components/FontLicenseModal.vue';
import { getServerVersion } from '@/api/version';
import { useI18n } from '@/i18n';
import { openExternalURL } from '@/features/settings/utils/externalLinks';

interface BundledFont {
  name: string;
  source: string;
  license: string;
}

const { t } = useI18n();
const version = ref('');
const githubRepoUrl = 'https://github.com/voilelab/plainshelf';

const bundledFonts: BundledFont[] = [
  {
    name: 'Noto Serif TC',
    source: 'https://fontsource.org/fonts/noto-serif-tc',
    license: '/licenses/noto-serif-tc-OFL-1.1.txt'
  },
  {
    name: 'Noto Sans TC',
    source: 'https://fontsource.org/fonts/noto-sans-tc',
    license: '/licenses/noto-sans-tc-OFL-1.1.txt'
  }
];
const selectedFontLicense = ref<BundledFont | null>(null);
const fontLicenseText = ref('');
const fontLicenseLoading = ref(false);
const fontLicenseError = ref('');
const fontLicenseCache = new Map<string, string>();
let fontLicenseRequest = 0;

function onRepositoryLinkClick(event: MouseEvent): void {
  event.preventDefault();
  void openExternalURL(githubRepoUrl);
}

function onBundledFontSourceClick(event: MouseEvent, url: string): void {
  event.preventDefault();
  void openExternalURL(url);
}

async function openFontLicense(font: BundledFont): Promise<void> {
  selectedFontLicense.value = font;
  fontLicenseError.value = '';

  const cachedText = fontLicenseCache.get(font.license);
  if (cachedText !== undefined) {
    fontLicenseText.value = cachedText;
    fontLicenseLoading.value = false;
    return;
  }

  const request = ++fontLicenseRequest;
  fontLicenseText.value = '';
  fontLicenseLoading.value = true;

  try {
    const response = await fetch(font.license);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const text = await response.text();
    fontLicenseCache.set(font.license, text);
    if (request === fontLicenseRequest) {
      fontLicenseText.value = text;
    }
  } catch {
    if (request === fontLicenseRequest) {
      fontLicenseError.value = t('settings.about.licenseLoadFailed');
    }
  } finally {
    if (request === fontLicenseRequest) {
      fontLicenseLoading.value = false;
    }
  }
}

function closeFontLicense(): void {
  fontLicenseRequest += 1;
  selectedFontLicense.value = null;
  fontLicenseLoading.value = false;
}

async function loadVersion(): Promise<void> {
  try {
    version.value = await getServerVersion();
  } catch {
    version.value = '';
  }
}

onMounted(() => {
  void loadVersion();
});
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.font-license-section {
  border-top: 1px solid var(--border);
  display: grid;
  gap: 10px;
  padding-top: 12px;
}

.font-license-item {
  align-items: start;
  display: grid;
  gap: 3px;
}

.font-license-links {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 2px;
}
</style>
