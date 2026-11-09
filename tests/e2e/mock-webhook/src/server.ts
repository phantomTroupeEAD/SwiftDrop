import express, { Request, Response } from 'express';
import cors from 'cors';
import crypto from 'node:crypto';
import path from 'node:path';

export interface WebhookRecord {
  id: string;
  tenant: 'alpha' | 'beta' | 'chaos';
  type: string;
  payload: any;
  hmacValid: boolean;
  receivedAt: string;
  signatureHeader?: string;
}

const SECRETS: Record<string, string> = {
  alpha: 'whsec_alpha_super_secret_test_key_01',
  beta: 'whsec_beta_super_secret_test_key_02',
};

export const eventStore: WebhookRecord[] = [];
export const chaosAttempts: { receivedAt: string; path: string }[] = [];

export function verifyHmac(rawBody: Buffer, signatureHeader: string | undefined, secret: string): boolean {
  if (!signatureHeader) return false;

  let signatureHex = signatureHeader;
  let payloadToSign: Buffer | string = rawBody;

  // Handle t=timestamp,v1=signature format if present
  if (signatureHeader.includes('v1=')) {
    const parts = signatureHeader.split(',');
    const timestamp = parts.find((p) => p.startsWith('t='))?.replace('t=', '');
    const sig = parts.find((p) => p.startsWith('v1='))?.replace('v1=', '');
    if (!timestamp || !sig) return false;
    signatureHex = sig;
    payloadToSign = `${timestamp}.${rawBody.toString('utf8')}`;
  }

  const expectedHex = crypto.createHmac('sha256', secret).update(payloadToSign).digest('hex');

  try {
    const sigBuffer = Buffer.from(signatureHex, 'hex');
    const expBuffer = Buffer.from(expectedHex, 'hex');
    if (sigBuffer.length !== expBuffer.length) return false;
    return crypto.timingSafeEqual(sigBuffer, expBuffer);
  } catch {
    return false;
  }
}

export const app = express();
app.use(cors());
// Capture raw bytes for cryptographic signature verification
app.use(express.raw({ type: 'application/json' }));
app.use(express.static(path.join(__dirname, '../public')));

function handleWebhook(tenant: 'alpha' | 'beta') {
  return (req: Request, res: Response) => {
    const rawBody = Buffer.isBuffer(req.body) ? req.body : Buffer.from(JSON.stringify(req.body || {}));
    const sigHeader = (req.headers['x-swiftdrop-signature'] || req.headers['x-hub-signature-256']) as string | undefined;
    const hmacValid = verifyHmac(rawBody, sigHeader, SECRETS[tenant]);

    let parsedPayload: any = {};
    try {
      parsedPayload = JSON.parse(rawBody.toString('utf8'));
    } catch {
      parsedPayload = { raw: rawBody.toString('utf8') };
    }

    const record: WebhookRecord = {
      id: parsedPayload.id || `evt_${Date.now()}_${Math.random().toString(36).substring(2, 7)}`,
      tenant,
      type: parsedPayload.type || 'parcel.unknown',
      payload: parsedPayload,
      hmacValid,
      receivedAt: new Date().toISOString(),
      signatureHeader: sigHeader,
    };

    eventStore.unshift(record);

    // ponytail: fail closed on bad HMAC in strict mode, but accept in permissive mode with flag
    if (!hmacValid && req.query.strict === 'true') {
      return res.status(401).json({ error: 'INVALID_HMAC_SIGNATURE', record });
    }

    return res.status(200).json({ received: true, tenant, id: record.id, hmacValid });
  };
}

// 1. Registered Merchant Receivers
app.post('/webhooks/merchant-alpha', handleWebhook('alpha'));
app.post('/webhooks/merchant-beta', handleWebhook('beta'));

// 2. Chaos Endpoint (Always returns 500 to trigger DLQ retry)
app.post('/webhooks/merchant-chaos', (req: Request, res: Response) => {
  chaosAttempts.unshift({ receivedAt: new Date().toISOString(), path: req.path });
  return res.status(500).json({ error: 'CHAOS_INJECTED_500_DEAD_LETTER_TRIGGER' });
});

// 3. Timeout Endpoint (Sleeps 10s to test client timeout)
app.post('/webhooks/merchant-slow', async (_req: Request, res: Response) => {
  await new Promise((resolve) => setTimeout(resolve, 10000));
  return res.status(200).json({ delayed: true });
});

// 4. API Endpoints for Playwright Assertions & Dashboard Polling
app.get('/api/events', (req: Request, res: Response) => {
  const tenant = req.query.tenant as string | undefined;
  if (tenant) {
    return res.json(eventStore.filter((e) => e.tenant === tenant));
  }
  return res.json({
    events: eventStore,
    counts: {
      alpha: eventStore.filter((e) => e.tenant === 'alpha').length,
      beta: eventStore.filter((e) => e.tenant === 'beta').length,
      chaos: chaosAttempts.length,
      total: eventStore.length,
    },
  });
});

app.post('/api/reset', (_req: Request, res: Response) => {
  eventStore.length = 0;
  chaosAttempts.length = 0;
  return res.json({ reset: true });
});

// 5. 1-Click Test Trigger (For live examiner demo)
app.post('/api/test-trigger', (req: Request, res: Response) => {
  const tenant = (req.query.tenant as 'alpha' | 'beta') || 'alpha';
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const samplePayload = {
    id: `evt_demo_${Date.now()}`,
    specversion: '1.0',
    type: 'parcel.delivered',
    source: 'https://api.swiftdrop.io/parcels',
    time: new Date().toISOString(),
    tenant_id: tenant === 'alpha' ? 'mrc_01HMERCHANTALPHA0001' : 'mrc_01HMERCHANTBETA00002',
    data: {
      tracking_number: `SWD-2026-${Math.floor(10000 + Math.random() * 90000)}`,
      status: 'DELIVERED',
      recipient: 'Dilshan Silva',
      location: 'Kandy Hub',
      proof_type: 'DIGITAL_SIGNATURE',
    },
  };

  const rawJson = JSON.stringify(samplePayload);
  const signatureHex = crypto
    .createHmac('sha256', SECRETS[tenant])
    .update(`${timestamp}.${rawJson}`)
    .digest('hex');

  eventStore.unshift({
    id: samplePayload.id,
    tenant,
    type: samplePayload.type,
    payload: samplePayload,
    hmacValid: true,
    receivedAt: new Date().toISOString(),
    signatureHeader: `t=${timestamp},v1=${signatureHex}`,
  });

  return res.json({ success: true, event: samplePayload });
});

app.get('/dashboard', (_req: Request, res: Response) => {
  res.sendFile(path.join(__dirname, '../public/dashboard.html'));
});

export function start(port: number = Number(process.env.PORT) || 9999) {
  return app.listen(port, '0.0.0.0', () => {
    console.log(`[mock-webhook] Receiver & Visual Dashboard running on http://0.0.0.0:${port}`);
  });
}

// Only auto-listen if run directly as entrypoint, not when imported by tests
if (process.argv[1]?.includes('server.ts') || process.argv[1]?.includes('server.js')) {
  start();
}
