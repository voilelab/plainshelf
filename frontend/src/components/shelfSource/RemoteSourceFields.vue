<template>
  <div class="remote-source-fields">
    <div class="remote-source-field">
      <label class="remote-source-label" :for="kindId">{{ t('shelfSource.kindLabel') }}</label>
      <select :id="kindId" v-model="kind" class="input" :disabled="disabled" data-testid="shelf-source-kind">
        <option value="plainshelf">{{ t('shelfSource.plainshelf') }}</option>
        <option value="webdav">{{ t('shelfSource.webdav') }}</option>
      </select>
    </div>

    <div class="remote-source-field">
      <label class="remote-source-label" :for="urlId">{{ t('shelfSource.urlLabel') }}</label>
      <input
        :id="urlId"
        v-model="url"
        class="input"
        type="url"
        inputmode="url"
        autocapitalize="none"
        autocorrect="off"
        spellcheck="false"
        :placeholder="t('shelfSource.urlPlaceholder')"
        :disabled="disabled"
        data-testid="shelf-source-url"
      />
      <span class="remote-source-hint">{{ t('shelfSource.urlHint') }}</span>
    </div>

    <div v-if="kind === 'webdav'" class="remote-source-field">
      <label class="remote-source-label" :for="usernameId">{{ t('shelfSource.usernameLabel') }}</label>
      <input
        :id="usernameId"
        v-model="username"
        class="input"
        type="text"
        autocomplete="off"
        autocapitalize="none"
        autocorrect="off"
        spellcheck="false"
        :disabled="disabled"
      />
    </div>

    <div class="remote-source-field">
      <label class="remote-source-label" :for="secretId">{{
        kind === 'webdav' ? t('shelfSource.passwordLabel') : t('shelfSource.tokenLabel')
      }}</label>
      <input
        :id="secretId"
        v-model="secret"
        class="input"
        type="password"
        autocomplete="off"
        autocapitalize="none"
        autocorrect="off"
        spellcheck="false"
        :disabled="disabled"
        data-testid="shelf-source-secret"
      />
      <span class="remote-source-hint">{{ t('shelfSource.secretHint') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useId } from 'vue';
import { useI18n } from '@/i18n';
import type { ShelfSourceKind } from '@/utils/shelfSourceCheck';

defineProps<{ disabled?: boolean }>();

const kind = defineModel<ShelfSourceKind>('kind', { required: true });
const url = defineModel<string>('url', { required: true });
const username = defineModel<string>('username', { required: true });
const secret = defineModel<string>('secret', { required: true });

const { t } = useI18n();
const kindId = `remote-source-kind-${useId()}`;
const urlId = `remote-source-url-${useId()}`;
const usernameId = `remote-source-username-${useId()}`;
const secretId = `remote-source-secret-${useId()}`;
</script>

<style scoped>
.remote-source-fields {
  display: grid;
  gap: 12px;
}

.remote-source-field {
  display: grid;
  gap: 6px;
}

.remote-source-label {
  font-size: 0.85rem;
  font-weight: 600;
}

.remote-source-hint {
  color: #64748b;
  font-size: 12px;
}
</style>
