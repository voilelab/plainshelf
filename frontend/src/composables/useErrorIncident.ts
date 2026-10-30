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

export function reportErrorIncident(err: unknown): void {
  const incident = (err as { incident?: unknown } | null | undefined)?.incident;
  if (typeof incident === 'string') {
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
