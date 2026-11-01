import { isCourierFull, failureNoteCharsLeft } from '../src/courier';

describe('display helpers (UT-FE-18..19)', () => {
  it('isCourierFull only greys out at or above capacity', () => {
    expect(isCourierFull(24, 25)).toBe(false);
    expect(isCourierFull(25, 25)).toBe(true);
    expect(isCourierFull(26, 25)).toBe(true);
    expect(isCourierFull(0, 0)).toBe(false); // unknown capacity: let the backend decide
  });
  it('OTHER needs a 15 character note, other reasons need none', () => {
    expect(failureNoteCharsLeft('OTHER', 'Gate locked')).toBe(4); // 11 chars (E-04 style)
    expect(failureNoteCharsLeft('OTHER', '   Gate was locked again   ')).toBe(0);
    expect(failureNoteCharsLeft('RECIPIENT_UNAVAILABLE', '')).toBe(0);
  });
});
