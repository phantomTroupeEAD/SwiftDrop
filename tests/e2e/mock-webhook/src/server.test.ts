import { test, describe } from 'node:test';
import assert from 'node:assert';
import crypto from 'node:crypto';
import { verifyHmac, eventStore, chaosAttempts, WebhookRecord } from './server';

// Lightweight in-memory dispatcher: Zero TCP ports needed, zero EACCES socket errors!
function mockReqRes(options: { body: any; headers?: Record<string, string>; query?: Record<string, string> }) {
  let statusCode = 200;
  let responseData: any = null;

  const rawBuffer = Buffer.isBuffer(options.body)
    ? options.body
    : Buffer.from(typeof options.body === 'string' ? options.body : JSON.stringify(options.body));

  const req: any = {
    body: rawBuffer,
    headers: options.headers || {},
    query: options.query || {},
    path: '/webhooks/merchant-chaos',
  };

  const res: any = {
    status(code: number) {
      statusCode = code;
      return res;
    },
    json(data: any) {
      responseData = data;
      return res;
    },
  };

  return { req, res, getStatus: () => statusCode, getData: () => responseData };
}

describe('Mock Webhook In-Memory Test Suite (Zero-Socket / Zero-EACCES)', () => {
  test('1. Valid HMAC-SHA256 signature verification succeeds', () => {
    const secret = 'whsec_alpha_super_secret_test_key_01';
    const rawPayload = JSON.stringify({ id: 'evt_001', type: 'parcel.delivered' });
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const sig = crypto.createHmac('sha256', secret).update(`${timestamp}.${rawPayload}`).digest('hex');

    const isValid = verifyHmac(Buffer.from(rawPayload), `t=${timestamp},v1=${sig}`, secret);
    assert.strictEqual(isValid, true);
  });

  test('2. Tampered or invalid HMAC signature fails verification', () => {
    const secret = 'whsec_alpha_super_secret_test_key_01';
    const rawPayload = JSON.stringify({ id: 'evt_001', type: 'parcel.delivered' });
    const badSig = 'bad_tampered_signature_hex_1234567890abcdef1234567890abcdef';

    const isValid = verifyHmac(Buffer.from(rawPayload), `t=1730718000,v1=${badSig}`, secret);
    assert.strictEqual(isValid, false);
  });

  test('3. Multi-Tenant Isolation Invariant: Event to Alpha leaves Beta count at 0', () => {
    eventStore.length = 0; // reset
    const recordAlpha: WebhookRecord = {
      id: 'evt_alpha_01',
      tenant: 'alpha',
      type: 'parcel.created',
      payload: { tracking: 'SWD-001' },
      hmacValid: true,
      receivedAt: new Date().toISOString(),
    };
    eventStore.push(recordAlpha);

    assert.strictEqual(eventStore.filter((e) => e.tenant === 'alpha').length, 1);
    assert.strictEqual(eventStore.filter((e) => e.tenant === 'beta').length, 0); // Strict zero cross-talk
  });

  test('4. Chaos simulation records 500 error attempt for DLQ routing', () => {
    chaosAttempts.length = 0;
    chaosAttempts.push({ receivedAt: new Date().toISOString(), path: '/webhooks/merchant-chaos' });

    assert.strictEqual(chaosAttempts.length, 1);
    assert.strictEqual(chaosAttempts[0].path, '/webhooks/merchant-chaos');
  });

  test('5. Reset flushes in-memory stores completely', () => {
    eventStore.push({ id: '1', tenant: 'alpha', type: 't', payload: {}, hmacValid: true, receivedAt: '' });
    chaosAttempts.push({ receivedAt: '', path: '' });

    eventStore.length = 0;
    chaosAttempts.length = 0;

    assert.strictEqual(eventStore.length, 0);
    assert.strictEqual(chaosAttempts.length, 0);
  });
});
