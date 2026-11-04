process.env.NODE_ENV = 'test';

import { test, describe, before, after } from 'node:test';
import assert from 'node:assert';
import http from 'node:http';
import crypto from 'node:crypto';
import { app, eventStore, chaosAttempts } from './server';

const TEST_PORT = 9998;
let server: http.Server;

before(async () => {
  await new Promise<void>((resolve) => {
    server = app.listen(TEST_PORT, () => resolve());
  });
});

after(async () => {
  await new Promise<void>((resolve) => {
    server.close(() => resolve());
  });
});

function post(path: string, body: any, headers: Record<string, string> = {}): Promise<{ status: number; data: any }> {
  return new Promise((resolve, reject) => {
    const rawData = Buffer.isBuffer(body) ? body : Buffer.from(typeof body === 'string' ? body : JSON.stringify(body));
    const req = http.request(
      `http://localhost:${TEST_PORT}${path}`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': rawData.length,
          ...headers,
        },
      },
      (res) => {
        let chunks: Buffer[] = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => {
          const bodyStr = Buffer.concat(chunks).toString('utf8');
          try {
            resolve({ status: res.statusCode || 0, data: JSON.parse(bodyStr) });
          } catch {
            resolve({ status: res.statusCode || 0, data: bodyStr });
          }
        });
      }
    );
    req.on('error', reject);
    req.write(rawData);
    req.end();
  });
}

describe('Mock Webhook Lean Test Suite', () => {
  test('1. Valid HMAC-SHA256 signature returns 200 and marks event verified', async () => {
    const secret = 'whsec_alpha_super_secret_test_key_01';
    const payload = { id: 'evt_001', type: 'parcel.delivered', data: { tracking: 'SWD-123' } };
    const raw = JSON.stringify(payload);
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const sig = crypto.createHmac('sha256', secret).update(`${timestamp}.${raw}`).digest('hex');

    const res = await post('/webhooks/merchant-alpha', raw, {
      'x-swiftdrop-signature': `t=${timestamp},v1=${sig}`,
    });

    assert.strictEqual(res.status, 200);
    assert.strictEqual(res.data.hmacValid, true);
    assert.strictEqual(res.data.tenant, 'alpha');
  });

  test('2. Multi-Tenant Isolation Invariant: Event to Alpha leaves Beta count at 0', async () => {
    eventStore.length = 0; // reset
    const payload = JSON.stringify({ id: 'evt_alpha_only', type: 'parcel.created' });

    await post('/webhooks/merchant-alpha', payload);

    assert.strictEqual(eventStore.filter((e) => e.tenant === 'alpha').length, 1);
    assert.strictEqual(eventStore.filter((e) => e.tenant === 'beta').length, 0); // Strict zero leakage!
  });

  test('3. Chaos endpoint returns HTTP 500 to trigger Event Worker DLQ routing', async () => {
    const res = await post('/webhooks/merchant-chaos', { test: true });
    assert.strictEqual(res.status, 500);
    assert.strictEqual(chaosAttempts.length >= 1, true);
  });

  test('4. Native 404 Invariant: Non-existent webhook path returns 404 Not Found', async () => {
    const res = await post('/webhooks/unregistered-path', { test: true });
    assert.strictEqual(res.status, 404);
  });

  test('5. Reset endpoint clears in-memory event stores cleanly', async () => {
    await post('/api/reset', {});
    assert.strictEqual(eventStore.length, 0);
    assert.strictEqual(chaosAttempts.length, 0);
  });
});
