<template>
  <div
    v-if="show"
    class="security-warning"
    :class="{ 'security-warning--collapsed': collapsed }"
  >
    <template v-if="collapsed">
      <button
        type="button"
        class="security-warning__badge"
        :aria-label="t('security.insecureWarning.expand')"
        :title="t('security.insecureWarning.expand')"
        @click="collapsed = false"
      >
        <span aria-hidden="true">⚠</span>
        {{ t('security.insecureWarning.badge') }}
      </button>
    </template>
    <template v-else>
      <div class="security-warning__panel" role="alert">
        <span class="security-warning__icon" aria-hidden="true">⚠</span>
        <div class="security-warning__text">
          <strong class="security-warning__title">{{ t('security.insecureWarning.title') }}</strong>
          <span>{{ t('security.insecureWarning.body') }}</span>
          <a
            class="security-warning__link"
            :href="DOCS_URL"
            target="_blank"
            rel="noopener noreferrer"
          >{{ t('security.insecureWarning.docsLink') }}</a>
        </div>
        <button
          type="button"
          class="security-warning__collapse"
          :aria-label="t('security.insecureWarning.collapse')"
          :title="t('security.insecureWarning.collapse')"
          @click="collapsed = true"
        >
          {{ t('security.insecureWarning.collapse') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { isInsecurePublicAccess } from '@/api/client';
import { useI18n } from '@/i18n';

const { t } = useI18n();

// The deployment guide that explains the loopback-only default and how to add
// an authentication boundary. Kept as a GitHub blob URL because the docs are
// not published to a website yet.
const DOCS_URL = 'https://github.com/voilelab/plainshelf/blob/main/docs/development/docker.md';

// Read once at setup: the flag is injected into the page before the bundle runs
// and never changes for the life of the document.
const show = isInsecurePublicAccess();

// Collapsing only minimizes the banner to a persistent badge, and the state is
// component-local so a reload brings the full warning back. There is
// deliberately no "dismiss forever": the point is that it cannot be silenced.
const collapsed = ref(false);
</script>

<style scoped>
.security-warning {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 2000;
  display: flex;
  justify-content: center;
  pointer-events: none;
}

.security-warning--collapsed {
  justify-content: flex-end;
  top: auto;
  bottom: 16px;
  right: 16px;
  left: auto;
}

.security-warning__panel {
  pointer-events: auto;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  width: 100%;
  padding: 10px 16px;
  background: #7f1d1d;
  color: #fff;
  border-bottom: 2px solid #b91c1c;
  font-size: 13px;
  line-height: 1.4;
}

.security-warning__icon {
  font-size: 16px;
  line-height: 1.4;
}

.security-warning__text {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 8px;
  flex: 1;
  min-width: 0;
}

.security-warning__title {
  font-weight: 700;
}

.security-warning__link {
  color: #fff;
  text-decoration: underline;
  white-space: nowrap;
}

.security-warning__collapse {
  pointer-events: auto;
  flex-shrink: 0;
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.6);
  border-radius: 6px;
  color: #fff;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}

.security-warning__collapse:hover {
  background: rgba(255, 255, 255, 0.15);
}

.security-warning__badge {
  pointer-events: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #7f1d1d;
  color: #fff;
  border: 1px solid #b91c1c;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
}

.security-warning__badge:hover {
  background: #991b1b;
}
</style>
