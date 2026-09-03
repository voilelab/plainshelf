import { readonly, ref } from 'vue';
import { newClientIncidentID } from '@/api/incident';

/**
 * The one error reference the user can see and copy.
 *
 * Module-level and a single slot, like useToasts: any layer can raise a
 * reference and the one notice mounted in App.vue renders it. A slot rather
 * than a queue because the number is only useful for the failure in front of
 * the user, and a retried read must not stack ten of them up.
 *
 * This is deliberately not the unified error component: the existing
 * `role="alert"` regions keep showing their own message, and this only adds the
 * number they have no way to reach.
 */
const incident = ref('');

/** Records the reference for a failure the user is about to be told about. */
export function reportIncident(id: string): void {
  const trimmed = id.trim();
  if (trimmed) {
    incident.value = trimmed;
  }
}

/**
 * The app-level Vue error hook's half of the reference.
 *
 * Every captured error reaches app.config.errorHandler - App.vue's
 * onErrorCaptured returns nothing, so it does not stop propagation - which
 * makes this the one place a frontend failure is numbered. The console line and
 * the notice quote the same number, so a report can be traced to the stack.
 */
export function reportUnhandledError(err: unknown, info: string): void {
  const id = newClientIncidentID();
  console.error(`Unhandled Vue error (${info}) [${id}]`, err);
  reportIncident(id);
}

/** Publishes the reference an error carries, when it carries one. */
export function reportErrorIncident(err: unknown): void {
  const incident = (err as { incident?: unknown } | null | undefined)?.incident;
  if (typeof incident === 'string') {
    reportIncident(incident);
  }
}

/**
 * The text an error region is about to show, publishing the error's reference
 * on the way.
 *
 * For the display sites that still hold the error object rather than only its
 * text. A pCloud setup failure - authorizing, checking a shelf, walking the
 * folder tree - talks to pCloud directly, so nothing between it and the user
 * would otherwise publish the number it minted.
 */
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
