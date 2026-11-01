import { ApiClient } from '../src/client';
import { memoryTokens } from '../src/memoryTokens';

type Call = { url: string; init: RequestInit };
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

function setup(handler: (c: Call, n: number) => Response | Promise<Response>, token: string | null = 'old') {
  const calls: Call[] = [];
  const tokens = memoryTokens(token);
  let signedOut = 0;
  const client = new ApiClient('/api/v1', tokens, async (url, init) => { calls.push({ url, init }); return handler({ url, init }, calls.length); }, () => signedOut++);
  return { client, calls, tokens, signedOut: () => signedOut };
}
const auth = (c: Call) => (c.init.headers as Record<string, string> | undefined)?.['Authorization'];

describe('ApiClient (UT-FE-11..17)', () => {
  it('sends the in-memory token and Idempotency-Key, returns data', async () => {
    const s = setup(() => json(200, { data: { assignment_id: 'a1' } }));
    const r = await s.client.post<{ assignment_id: string }>('/dispatch/assignments', { parcel_id: 'p', courier_id: 'c' }, { idempotencyKey: 'k1' });
    expect(r.assignment_id).toBe('a1');
    const h = s.calls[0].init.headers as Record<string, string>;
    expect(h['Authorization']).toBe('Bearer old');
    expect(h['Idempotency-Key']).toBe('k1');
    expect(s.calls[0].init.body).toBe('{"parcel_id":"p","courier_id":"c"}');
  });

  it('on 401 refreshes once and repeats the request once with the new token and same key', async () => {
    const s = setup((c) => {
      if (c.url.endsWith('/auth/refresh')) return json(200, { data: { access_token: 'new' } });
      return auth(c) === 'Bearer new' ? json(200, { data: 'ok' }) : json(401, {});
    });
    expect(await s.client.post('/zones', { a: 1 }, { idempotencyKey: 'k9' })).toBe('ok');
    expect(s.calls.map((c) => c.url)).toEqual(['/api/v1/zones', '/api/v1/auth/refresh', '/api/v1/zones']);
    expect((s.calls[2].init.headers as Record<string, string>)['Idempotency-Key']).toBe('k9');
    expect(s.tokens.get()).toBe('new');
  });

  it('5 parallel 401s cause a single refresh call (race condition)', async () => {
    const s = setup(async (c) => {
      if (c.url.endsWith('/auth/refresh')) { await new Promise((r) => setTimeout(r, 10)); return json(200, { data: { access_token: 'new' } }); }
      return auth(c) === 'Bearer new' ? json(200, { data: 1 }) : json(401, {});
    });
    await Promise.all(Array.from({ length: 5 }, () => s.client.get('/dispatch/board')));
    expect(s.calls.filter((c) => c.url.endsWith('/auth/refresh'))).toHaveLength(1);
  });

  it('when refresh fails: clears the token, signs out, throws 401', async () => {
    const s = setup((c) => (c.url.endsWith('/auth/refresh') ? json(401, {}) : json(401, {})));
    await expect(s.client.get('/dispatch/board')).rejects.toMatchObject({ status: 401 });
    expect(s.tokens.get()).toBeNull();
    expect(s.signedOut()).toBe(1);
  });

  it('does not loop: a second 401 after refresh is returned as an error', async () => {
    const s = setup((c) => (c.url.endsWith('/auth/refresh') ? json(200, { data: { access_token: 'new' } }) : json(401, {})));
    await expect(s.client.get('/x')).rejects.toMatchObject({ status: 401 });
    expect(s.calls).toHaveLength(3);
  });

  it('403 is not retried and not refreshed (authorization failure)', async () => {
    const s = setup(() => json(403, { error: { code: 'ACCESS_DENIED', message: 'Forbidden' } }));
    await expect(s.client.get('/admin/zones')).rejects.toMatchObject({ status: 403, code: 'ACCESS_DENIED' });
    expect(s.calls).toHaveLength(1);
  });

  it('network failure becomes a NETWORK_ERROR ApiError (dependency failure)', async () => {
    const s = setup(() => { throw new TypeError('Failed to fetch'); });
    await expect(s.client.get('/x')).rejects.toMatchObject({ status: 0, code: 'NETWORK_ERROR' });
  });
});
