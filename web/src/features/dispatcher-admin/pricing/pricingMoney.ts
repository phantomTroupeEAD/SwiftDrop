// Money for the pricing screen. Amounts travel as integer LKR cents (never floats). This file only
// turns what an admin types ("450.00") into cents and cents back into text. It never works out a
// price: the server calculates every quote.

const AMOUNT = /^\d{1,10}(\.\d{1,2})?$/;
// Commas are accepted only as thousands separators: 1,250.00 yes, 12,3,4 no.
const GROUPED = /^\d{1,3}(,\d{3})+(\.\d{1,2})?$/;

/** "450", "450.5" or "450.50" become 45000, 45050 and 45050. Returns null if the text is not an amount. */
export function parseLkrToCents(text: string): number | null {
  let t = text.trim();
  if (GROUPED.test(t)) t = t.replace(/,/g, '');
  if (!AMOUNT.test(t)) return null;
  const [whole, frac = ''] = t.split('.');
  return Number(whole) * 100 + Number(frac.padEnd(2, '0'));
}

/** 45000 becomes "450.00", for filling an edit box. */
export function centsToInput(cents: number): string {
  const whole = Math.trunc(cents / 100);
  const frac = cents % 100;
  return `${whole}.${String(frac).padStart(2, '0')}`;
}

/** 45000 becomes "LKR 450.00". Display only. */
export function formatLkr(cents: number): string {
  const whole = Math.trunc(cents / 100);
  const frac = cents % 100;
  return `LKR ${whole.toLocaleString('en-US')}.${String(frac).padStart(2, '0')}`;
}
