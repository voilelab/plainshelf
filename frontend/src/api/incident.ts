const INCIDENT_ALPHABET = '23456789ABCDEFGHJKMNPQRSTVWXYZ';
const INCIDENT_LENGTH = 8;

export function newClientIncidentID(): string {
  return `c-${randomSymbols(INCIDENT_LENGTH)}`;
}

function randomSymbols(length: number): string {
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

  for (let i = 0; i < length; i += 1) {
    buffer[i] = Math.floor(Math.random() * 256);
  }
  return buffer;
}
