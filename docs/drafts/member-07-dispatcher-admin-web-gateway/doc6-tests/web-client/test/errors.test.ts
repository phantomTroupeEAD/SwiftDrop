import { toApiError } from '../src/errors';

describe('toApiError (UT-FE-01..04)', () => {
  it('maps a 409 envelope with code, message and correlation id', () => {
    const e = toApiError(409, { error: { code: 'COURIER_CAPACITY_EXCEEDED', message: 'Courier has reached maximum capacity', correlation_id: 'corr_1' } });
    expect(e).toMatchObject({ status: 409, code: 'COURIER_CAPACITY_EXCEEDED', message: 'Courier has reached maximum capacity', correlationId: 'corr_1' });
  });
  it('collects field errors from details[] for 422', () => {
    const e = toApiError(422, { error: { code: 'VALIDATION_FAILED', message: 'Invalid', details: [{ field: 'zone_code', issue: 'must start with ZONE_' }, { field: 'name' }] } });
    expect(e.fieldErrors).toEqual({ zone_code: 'must start with ZONE_' });
  });
  it('never shows server text for 5xx (INV-10)', () => {
    const e = toApiError(500, { error: { code: 'INTERNAL_ERROR', message: 'pq: relation "x" does not exist', correlation_id: 'c9' } });
    expect(e.message).toBe('Something went wrong. Please try again.');
    expect(e.correlationId).toBe('c9');
  });
  it('gives safe defaults for empty bodies and network errors', () => {
    expect(toApiError(403, null).message).toBe('You do not have permission for this action.');
    expect(toApiError(413, 'oops').message).toBe('File is too large (maximum 10 MB).');
    expect(toApiError(0, null)).toMatchObject({ code: 'NETWORK_ERROR', status: 0 });
    expect(toApiError(418, {}).code).toBe('HTTP_418');
  });
});
