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

const DOCS_URL = 'https://github.com/voilelab/plainshelf/blob/main/docs/development/docker.md';

const show = isInsecurePublicAccess();

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
  background: var(--danger-strong);
  color: var(--on-accent);
  border-bottom: 2px solid var(--danger-text);
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
  color: var(--on-accent);
  text-decoration: underline;
  white-space: nowrap;
}

.security-warning__collapse {
  pointer-events: auto;
  flex-shrink: 0;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--on-accent) 60%, transparent);
  border-radius: 6px;
  color: var(--on-accent);
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}

.security-warning__collapse:hover {
  background: color-mix(in srgb, var(--on-accent) 15%, transparent);
}

.security-warning__badge {
  pointer-events: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--danger-strong);
  color: var(--on-accent);
  border: 1px solid var(--danger-text);
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  cursor: pointer;
  box-shadow: 0 2px 8px rgb(var(--shadow-rgb) / 0.25);
}

.security-warning__badge:hover {
  background: color-mix(in srgb, var(--danger-strong) 85%, var(--danger));
}
</style>
