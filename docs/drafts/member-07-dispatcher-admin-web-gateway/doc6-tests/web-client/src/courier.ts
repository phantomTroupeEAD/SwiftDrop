// Display-only helpers for the dispatch board and courier forms. The backend still decides.
export function isCourierFull(active: number, max: number): boolean {
  return max > 0 && active >= max;
}

export const OTHER_NOTE_MIN = 15; // ADR-004

export function failureNoteCharsLeft(reason: string, note: string): number {
  if (reason !== 'OTHER') return 0;
  return Math.max(0, OTHER_NOTE_MIN - note.trim().length);
}
