import { readonly, ref } from 'vue';
import { newClientIncidentID } from '@/api/incident';

const incident = ref('');

export function reportIncident(id: string): void {
  const trimmed = id.trim();
  if (trimmed) {
    incident.value = trimmed;
  }
}

export function reportUnhandledError(err: unknown, info: string): void {
  const id = newClientIncidentID();
  console.error(`Unhandled Vue error (${info}) [${id}]`, err);
  reportIncident(id);
}

export function isRefusalStatus(status: number | undefined): boolean {
  return status !== undefined && status >= 400 && status < 500;
}

export function reportErrorIncident(err: unknown): void {
  const { incident, status } = (err ?? {}) as { incident?: unknown; status?: number };
  if (typeof incident === 'string' && !isRefusalStatus(status)) {
    reportIncident(incident);
  }
}

export function surfaceError(err: unknown): string {
  reportErrorIncident(err);
  return err instanceof Error ? err.message : String(err);
}

export function useErrorIncident() {
  function dismissIncident(): void {
    incident.value = '';
  }

  return { incident: readonly(incident), dismissIncident };
}
