import { computed, onScopeDispose, ref, watch, type Ref } from 'vue';
import { useI18n } from '@/i18n';
import {
  shelfSourceCheckMessageKey,
  shelfSourceNeedsSecret,
  type ShelfSourceCheck,
  type ShelfSourceKind
} from '@/utils/shelfSourceCheck';

interface SourceCheckFields {
  kind: Ref<ShelfSourceKind>;
  url: Ref<string>;
  username: Ref<string>;
  secret: Ref<string>;
}

interface SourceCheckParams {
  source: ShelfSourceKind;
  url: string;
  username: string;
  secret: string;
}

export type SourceCheckStatus = 'idle' | 'checking' | 'ok' | 'error';

const SOURCE_CHECK_DELAY_MS = 600;

export function useSourceCheck(
  fields: SourceCheckFields,
  check: (params: SourceCheckParams) => Promise<ShelfSourceCheck>
) {
  const { t } = useI18n();
  const status = ref<SourceCheckStatus>('idle');
  const books = ref(0);
  const error = ref('');
  let revision = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const params = (): SourceCheckParams => {
    const source = fields.kind.value;
    return {
      source,
      url: fields.url.value.trim(),
      username: source === 'webdav' ? fields.username.value.trim() : '',
      secret: fields.secret.value.trim()
    };
  };

  const ready = computed(() => {
    const p = params();
    return p.url.length > 0 && (!shelfSourceNeedsSecret(p.source, p.username) || p.secret.length > 0);
  });

  function cancelPending(): void {
    if (timer !== undefined) {
      clearTimeout(timer);
      timer = undefined;
    }
  }

  async function run(): Promise<boolean> {
    cancelPending();
    const current = ++revision;
    status.value = 'checking';
    error.value = '';
    try {
      const result = await check(params());
      if (current !== revision) {
        return false;
      }
      const key = shelfSourceCheckMessageKey(result);
      if (key) {
        status.value = 'error';
        error.value = t(key);
        return false;
      }
      books.value = result.books;
      status.value = 'ok';
      return true;
    } catch (err) {
      if (current !== revision) {
        return false;
      }
      status.value = 'error';
      error.value = err instanceof Error ? err.message : t('shelfSource.unavailable');
      return false;
    }
  }

  watch(
    [fields.kind, fields.url, fields.username, fields.secret],
    () => {
      revision++;
      cancelPending();
      status.value = 'idle';
      error.value = '';
      if (ready.value) {
        timer = setTimeout(() => void run(), SOURCE_CHECK_DELAY_MS);
      }
    },
    { flush: 'sync' }
  );

  onScopeDispose(cancelPending);

  async function ensureChecked(): Promise<boolean> {
    if (status.value === 'ok') {
      return true;
    }
    return ready.value ? run() : false;
  }

  function reset(): void {
    revision++;
    cancelPending();
    status.value = 'idle';
    error.value = '';
  }

  return { status, books, error, ready, ensureChecked, reset };
}
