/**
 * Incident IDs for failures no PlainShelf server answered.
 *
 * The alphabet and length mirror `internal/logutil/requestid.go` so a number a
 * user reads off the screen has the same shape whichever side minted it, and
 * the `c-` prefix is what says the server never saw this one — nobody should
 * search the server log for it.
 */
const INCIDENT_ALPHABET = '23456789ABCDEFGHJKMNPQRSTVWXYZ';
const INCIDENT_LENGTH = 8;

/** Mints a client-side incident ID, e.g. `c-K7MQ4XZB`. */
export function newClientIncidentID(): string {
  return `c-${randomSymbols(INCIDENT_LENGTH)}`;
}

function randomSymbols(length: number): string {
  // Bytes at or above the bound are discarded rather than folded, which would
  // hand the first 256 % 30 symbols a larger share than the rest.
  const bound = 256 - (256 % INCIDENT_ALPHABET.length);
  const symbols: string[] = [];

  while (symbols.length < length) {
    for (const byte of randomBytes(length)) {
      if (byte >= bound) {
        continue;
      }
      symbols.push(INCIDENT_ALPHABET[byte % INCIDENT_ALPHABET.length]);
      if (symbols.length === length) {
        break;
      }
    }
  }

  return symbols.join('');
}

function randomBytes(length: number): Uint8Array {
  const buffer = new Uint8Array(length);
  const source = globalThis.crypto;

  if (typeof source?.getRandomValues === 'function') {
    source.getRandomValues(buffer);
    return buffer;
  }

  // No Web Crypto (an old Android WebView). An incident ID only has to name one
  // failure inside one log, so a weaker source is still enough for that.
  for (let i = 0; i < length; i += 1) {
    buffer[i] = Math.floor(Math.random() * 256);
  }
  return buffer;
}
