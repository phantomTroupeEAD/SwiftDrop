import { IdempotentAction } from '../src/idempotency';

describe('IdempotentAction (UT-FE-07..10)', () => {
  const counter = () => { let n = 0; return () => `key-${++n}`; };
  it('reuses the same key when the same input is retried', () => {
    const a = new IdempotentAction<{ parcelId: string; courierId: string }>(counter());
    const k1 = a.keyFor({ parcelId: 'p1', courierId: 'c1' });
    expect(a.keyFor({ parcelId: 'p1', courierId: 'c1' })).toBe(k1);
  });
  it('makes a new key when the input changes', () => {
    const a = new IdempotentAction<{ courierId: string }>(counter());
    expect(a.keyFor({ courierId: 'c1' })).toBe('key-1');
    expect(a.keyFor({ courierId: 'c2' })).toBe('key-2');
  });
  it('drops the key after success', () => {
    const a = new IdempotentAction<string>(counter());
    a.keyFor('x');
    a.succeeded();
    expect(a.keyFor('x')).toBe('key-2');
  });
  it('default keys are UUID v4', () => {
    expect(new IdempotentAction<string>().keyFor('x')).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  });
});
