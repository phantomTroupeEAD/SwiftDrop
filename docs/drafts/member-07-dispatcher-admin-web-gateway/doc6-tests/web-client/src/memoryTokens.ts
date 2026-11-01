import type { TokenStore } from './client';
// Access token lives in memory only (ADR-003, INV-11). No browser storage.
export function memoryTokens(initial: string | null = null): TokenStore {
  let token = initial;
  return { get: () => token, set: (t) => { token = t; } };
}
