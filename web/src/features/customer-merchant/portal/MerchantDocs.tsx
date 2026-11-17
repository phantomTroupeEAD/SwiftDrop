import React, { useState } from 'react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from '@/shared/components/ui/tabs';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/shared/components/ui/table';
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from '@/shared/components/ui/accordion';
import {
  Check,
  Copy,
  Key,
  Webhook,
  Package,
  ShieldCheck,
  AlertCircle,
} from 'lucide-react';
import { PortalLayout } from './PortalLayout';

interface CodeBlockProps {
  code: string;
  language?: string;
  testId?: string;
}

const CodeBlock: React.FC<CodeBlockProps> = ({ code, language = 'text', testId }) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback if clipboard API is unavailable
    }
  };

  return (
    <div className="relative my-3">
      <div className="flex items-center justify-between px-3 py-1.5 bg-surface-subtle border-t border-x border-border rounded-t-md text-xs text-text-muted font-mono">
        <span>{language}</span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs gap-1"
          onClick={handleCopy}
          data-testid={testId || 'code-copy-btn'}
        >
          {copied ? (
            <>
              <Check className="w-3.5 h-3.5 text-status-success-ink" aria-hidden="true" />
              <span>Copied</span>
            </>
          ) : (
            <>
              <Copy className="w-3.5 h-3.5" aria-hidden="true" />
              <span>Copy</span>
            </>
          )}
        </Button>
      </div>
      <pre className="overflow-x-auto rounded-b-md border border-border bg-surface p-3.5 text-xs font-mono tabular-nums text-foreground leading-relaxed">
        <code>{code}</code>
      </pre>
      {copied && <span role="status" className="sr-only">Code copied to clipboard</span>}
    </div>
  );
};

export const MerchantDocs: React.FC = () => {
  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://api.swiftdrop.local';
  const apiBase = `${origin}/api/v1`;

  const curlAuthSnippet = `curl -X GET "${apiBase}/merchants/me" \\
  -H "X-API-Key: sd_live_01HMERCH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" \\
  -H "Accept: application/json"`;

  const nodeAuthSnippet = `const response = await fetch("${apiBase}/merchants/me", {
  method: "GET",
  headers: {
    "X-API-Key": "sd_live_01HMERCH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "Accept": "application/json"
  }
});
const profile = await response.json();
console.log("Merchant:", profile.data.business_name);`;

  const pythonAuthSnippet = `import requests

url = "${apiBase}/merchants/me"
headers = {
    "X-API-Key": "sd_live_01HMERCH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "Accept": "application/json"
}

response = requests.get(url, headers=headers)
print(response.json())`;

  const orderPayload = `{
  "merchant_reference": "ORD-2026-98214",
  "service_level": "EXPRESS",
  "pickup": {
    "name": "Colombo Tech Stores",
    "phone": "+94770000004",
    "address": "42 Galle Road",
    "city": "Colombo",
    "postal_code": "00300"
  },
  "dropoff": {
    "name": "Nimal Fernando",
    "phone": "+94770000005",
    "address": "12 Peradeniya Road",
    "city": "Kandy",
    "postal_code": "20000"
  },
  "parcel": {
    "weight_kg": 1.75,
    "description": "Electronics & Accessories"
  }
}`;

  const curlOrderSnippet = `curl -X POST "${apiBase}/merchant/orders" \\
  -H "X-API-Key: sd_live_01HMERCH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" \\
  -H "Idempotency-Key: $(uuidgen)" \\
  -H "Content-Type: application/json" \\
  -d '${orderPayload.replace(/\n/g, '')}'`;

  const nodeOrderSnippet = `import { randomUUID } from "crypto";

const payload = ${orderPayload};

const response = await fetch("${apiBase}/merchant/orders", {
  method: "POST",
  headers: {
    "X-API-Key": "sd_live_01HMERCH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "Idempotency-Key": randomUUID(),
    "Content-Type": "application/json"
  },
  body: JSON.stringify(payload)
});

const result = await response.json();
console.log("Tracking Number:", result.data.tracking_number);`;

  const pythonOrderSnippet = `import uuid
import requests

payload = ${orderPayload}

headers = {
    "X-API-Key": "sd_live_01HMERCH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "Idempotency-Key": str(uuid.uuid4()),
    "Content-Type": "application/json"
}

response = requests.post("${apiBase}/merchant/orders", json=payload, headers=headers)
print("Tracking:", response.json()["data"]["tracking_number"])`;

  const webhookVerifyNode = `import crypto from "crypto";

function verifyWebhookSignature(payloadBody, receivedSignature, secret) {
  const expectedSignature = crypto
    .createHmac("sha256", secret)
    .update(payloadBody, "utf8")
    .digest("hex");
  return crypto.timingSafeEqual(
    Buffer.from(receivedSignature),
    Buffer.from(expectedSignature)
  );
}`;

  const webhookVerifyPython = `import hmac
import hashlib

def verify_webhook_signature(payload_bytes, received_signature, secret):
    expected = hmac.new(secret.encode("utf-8"), payload_bytes, hashlib.sha256).hexdigest()
    return hmac.compare_digest(received_signature, expected)`;

  const webhookPayloadSample = `{
  "event_id": "evt_01HMERCH00000000000001",
  "event_type": "parcel.delivered",
  "occurred_at": "2026-11-17T18:45:00Z",
  "payload": {
    "parcel_id": "018f4a7c-1004-7000-8000-000000000004",
    "tracking_number": "SD-20261117-00042",
    "merchant_reference": "ORD-2026-98214",
    "status": "DELIVERED",
    "recipient_name": "Nimal Fernando",
    "delivered_at": "2026-11-17T18:44:52Z"
  }
}`;

  return (
    <PortalLayout title="API & Integration Guide" profile="merchant">
      <div className="space-y-4" data-testid="merchant-docs-container">
        {/* Overview Section */}
        <section className="m6-card">
          <div className="flex flex-wrap items-center gap-2 mb-3">
            <Badge variant="success">Production & Sandbox</Badge>
            <Badge variant="outline">HTTPS REST</Badge>
            <Badge variant="outline">JSON Payloads</Badge>
            <Badge variant="secondary">Idempotency Supported</Badge>
          </div>
          <h2>Overview & Quickstart</h2>
          <p className="text-sm text-text-muted">
            Connect your storefront, automate parcel bookings, and subscribe to real-time delivery status updates.
            This guide details HTTP endpoints, authentication headers, request schemas, and HMAC webhook verification.
          </p>
          <div className="m6-actions">
            <Button asChild variant="outline" size="sm">
              <a href="#authentication">Authentication</a>
            </Button>
            <Button asChild variant="outline" size="sm">
              <a href="#orders-api">B2B Parcel Orders</a>
            </Button>
            <Button asChild variant="outline" size="sm">
              <a href="#webhooks">Webhooks</a>
            </Button>
            <Button asChild variant="outline" size="sm">
              <a href="#error-codes">Error Reference</a>
            </Button>
          </div>
        </section>

        {/* Section 1: Authentication */}
        <section id="authentication" className="m6-card scroll-mt-6">
          <div className="flex items-center gap-2 mb-2">
            <Key className="w-5 h-5 text-primary" aria-hidden="true" />
            <h2 className="!mb-0">1. Authentication & API Keys</h2>
          </div>
          <p className="text-sm text-text-muted">
            Authenticate programmatic B2B API calls using high-entropy API credentials.
            All requests must include your API key in the <code className="font-mono bg-surface-subtle px-1.5 py-0.5 rounded-sm border border-border">X-API-Key</code> request header.
          </p>

          <div className="rounded-md border border-border bg-surface-subtle p-3 my-3 space-y-1.5">
            <div className="flex items-center gap-2 font-medium text-foreground text-sm">
              <ShieldCheck className="w-4 h-4 text-primary" aria-hidden="true" />
              <span>Security Guidelines</span>
            </div>
            <ul className="list-disc list-inside space-y-1 text-xs text-text-muted pl-1">
              <li>Keep API keys server-side; never expose them in client web apps or public repositories.</li>
              <li>Do not pass API keys in query parameters (<code className="font-mono">?api_key=</code>); SwiftDrop rejects them for security.</li>
              <li>Send either <code className="font-mono">X-API-Key</code> or <code className="font-mono">Authorization: Bearer</code>, never both (triggers <code className="font-mono">400 AMBIGUOUS_CREDENTIALS</code>).</li>
              <li>Each key possesses explicit scopes: <code className="font-mono">parcels:create</code> and <code className="font-mono">parcels:read</code>.</li>
            </ul>
          </div>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Verify Your API Credentials</h3>
            <Tabs defaultValue="curl" className="w-full">
              <TabsList>
                <TabsTrigger value="curl">cURL</TabsTrigger>
                <TabsTrigger value="node">Node.js</TabsTrigger>
                <TabsTrigger value="python">Python</TabsTrigger>
              </TabsList>
              <TabsContent value="curl">
                <CodeBlock code={curlAuthSnippet} language="bash" testId="auth-curl-code" />
              </TabsContent>
              <TabsContent value="node">
                <CodeBlock code={nodeAuthSnippet} language="javascript" testId="auth-node-code" />
              </TabsContent>
              <TabsContent value="python">
                <CodeBlock code={pythonAuthSnippet} language="python" testId="auth-python-code" />
              </TabsContent>
            </Tabs>
          </div>
        </section>

        {/* Section 2: B2B Order Creation */}
        <section id="orders-api" className="m6-card scroll-mt-6">
          <div className="flex items-center gap-2 mb-2">
            <Package className="w-5 h-5 text-primary" aria-hidden="true" />
            <h2 className="!mb-0">2. B2B Parcel Order Creation</h2>
          </div>
          <p className="text-sm text-text-muted">
            Book parcel shipments programmatically from your inventory or order management system.
          </p>

          <div className="flex items-center gap-2 font-mono text-xs my-2">
            <Badge variant="default">POST</Badge>
            <span className="text-foreground">{apiBase}/merchant/orders</span>
          </div>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Required Headers</h3>
            <div className="m6-scroll" tabIndex={0} role="region" aria-label="Order headers table">
              <Table className="m6-table" aria-label="Order headers">
                <TableHeader>
                  <TableRow>
                    <TableHead>Header</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Description</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell className="font-mono text-xs">X-API-Key</TableCell>
                    <TableCell>String</TableCell>
                    <TableCell>Your active merchant API key with <code className="font-mono">parcels:create</code> scope.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">Idempotency-Key</TableCell>
                    <TableCell>UUID</TableCell>
                    <TableCell>
                      A unique UUID v4. Protects against duplicate billing or duplicate bookings on network retries.
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">Content-Type</TableCell>
                    <TableCell>String</TableCell>
                    <TableCell><code className="font-mono">application/json</code></TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </div>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Request Payload Schema</h3>
            <Accordion type="single" collapsible className="w-full">
              <AccordionItem value="schema-details">
                <AccordionTrigger className="text-xs">
                  View payload parameter definitions
                </AccordionTrigger>
                <AccordionContent>
                  <div className="m6-scroll" tabIndex={0} role="region" aria-label="Payload schema table">
                    <Table className="m6-table" aria-label="Payload schema">
                      <TableHeader>
                        <TableRow>
                          <TableHead>Field</TableHead>
                          <TableHead>Type</TableHead>
                          <TableHead>Required</TableHead>
                          <TableHead>Validation Rule</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        <TableRow>
                          <TableCell className="font-mono text-xs">merchant_reference</TableCell>
                          <TableCell>string</TableCell>
                          <TableCell>Optional</TableCell>
                          <TableCell>Your internal order number (e.g. ORD-98214).</TableCell>
                        </TableRow>
                        <TableRow>
                          <TableCell className="font-mono text-xs">service_level</TableCell>
                          <TableCell>string</TableCell>
                          <TableCell>Optional</TableCell>
                          <TableCell><code className="font-mono">STANDARD</code> or <code className="font-mono">EXPRESS</code>.</TableCell>
                        </TableRow>
                        <TableRow>
                          <TableCell className="font-mono text-xs">pickup</TableCell>
                          <TableCell>object</TableCell>
                          <TableCell>Required</TableCell>
                          <TableCell>Requires <code className="font-mono">name</code>, <code className="font-mono">phone</code>, <code className="font-mono">address</code>, <code className="font-mono">city</code>.</TableCell>
                        </TableRow>
                        <TableRow>
                          <TableCell className="font-mono text-xs">dropoff</TableCell>
                          <TableCell>object</TableCell>
                          <TableCell>Required</TableCell>
                          <TableCell>Requires <code className="font-mono">name</code>, <code className="font-mono">phone</code>, <code className="font-mono">address</code>, <code className="font-mono">city</code>.</TableCell>
                        </TableRow>
                        <TableRow>
                          <TableCell className="font-mono text-xs">parcel.weight_kg</TableCell>
                          <TableCell>number</TableCell>
                          <TableCell>Required</TableCell>
                          <TableCell>Positive decimal in kilograms (must be &gt; 0).</TableCell>
                        </TableRow>
                      </TableBody>
                    </Table>
                  </div>
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </div>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Code Example</h3>
            <Tabs defaultValue="curl" className="w-full">
              <TabsList>
                <TabsTrigger value="curl">cURL</TabsTrigger>
                <TabsTrigger value="node">Node.js</TabsTrigger>
                <TabsTrigger value="python">Python</TabsTrigger>
              </TabsList>
              <TabsContent value="curl">
                <CodeBlock code={curlOrderSnippet} language="bash" testId="order-curl-code" />
              </TabsContent>
              <TabsContent value="node">
                <CodeBlock code={nodeOrderSnippet} language="javascript" testId="order-node-code" />
              </TabsContent>
              <TabsContent value="python">
                <CodeBlock code={pythonOrderSnippet} language="python" testId="order-python-code" />
              </TabsContent>
            </Tabs>
          </div>

          <div className="rounded-md border border-border bg-surface-subtle p-3 mt-4 space-y-1 text-xs">
            <span className="font-medium text-foreground">Idempotency & Replay Guarantee</span>
            <p className="text-text-muted">
              If a request fails due to network timeout, you can safely re-send the exact same payload with the original <code className="font-mono">Idempotency-Key</code>.
              SwiftDrop detects the key and returns the cached parcel result with header <code className="font-mono">Idempotent-Replay: true</code> without double-billing.
            </p>
          </div>
        </section>

        {/* Section 3: Webhooks & Delivery Events */}
        <section id="webhooks" className="m6-card scroll-mt-6">
          <div className="flex items-center gap-2 mb-2">
            <Webhook className="w-5 h-5 text-primary" aria-hidden="true" />
            <h2 className="!mb-0">3. Webhook Subscriptions & Event Catalog</h2>
          </div>
          <p className="text-sm text-text-muted">
            Receive instant HTTP callbacks whenever a parcel transitions across transit milestones.
            Register webhook endpoints in the <strong>Webhooks</strong> tab or via the API. SwiftDrop delivers events as HTTPS POST requests with JSON payloads.
          </p>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Supported Lifecycle Events</h3>
            <div className="m6-scroll" tabIndex={0} role="region" aria-label="Webhook events table">
              <Table className="m6-table" aria-label="Webhook event catalog">
                <TableHeader>
                  <TableRow>
                    <TableHead>Event Type</TableHead>
                    <TableHead>Description</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.created</TableCell>
                    <TableCell>Order ingested; parcel aggregate created in the system.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.picked_up</TableCell>
                    <TableCell>Courier has physically picked up the parcel from your location.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.in_transit</TableCell>
                    <TableCell>Parcel reached a regional distribution hub or sorting facility.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.out_for_delivery</TableCell>
                    <TableCell>Dispatched with courier for final-mile delivery.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.delivered</TableCell>
                    <TableCell>Successfully handed over to recipient with proof recorded.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.attempt_failed</TableCell>
                    <TableCell>Delivery attempt failed (e.g. recipient unavailable).</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">parcel.returned</TableCell>
                    <TableCell>Parcel returned to merchant after maximum attempts reached.</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </div>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Sample Webhook Payload</h3>
            <CodeBlock code={webhookPayloadSample} language="json" testId="webhook-payload-sample" />
          </div>

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Verifying HMAC-SHA256 Signatures</h3>
            <p className="text-xs text-text-muted">
              Every webhook request includes an <code className="font-mono">X-SwiftDrop-Signature</code> header generated using HMAC-SHA256 with your endpoint secret (<code className="font-mono">whsec_...</code>).
            </p>
            <Tabs defaultValue="node" className="w-full">
              <TabsList>
                <TabsTrigger value="node">Node.js</TabsTrigger>
                <TabsTrigger value="python">Python</TabsTrigger>
              </TabsList>
              <TabsContent value="node">
                <CodeBlock code={webhookVerifyNode} language="javascript" testId="verify-node-code" />
              </TabsContent>
              <TabsContent value="python">
                <CodeBlock code={webhookVerifyPython} language="python" testId="verify-python-code" />
              </TabsContent>
            </Tabs>
          </div>
        </section>

        {/* Section 4: Error Handling */}
        <section id="error-codes" className="m6-card scroll-mt-6">
          <div className="flex items-center gap-2 mb-2">
            <AlertCircle className="w-5 h-5 text-primary" aria-hidden="true" />
            <h2 className="!mb-0">4. Error Reference & Status Codes</h2>
          </div>
          <p className="text-sm text-text-muted">
            SwiftDrop uses conventional HTTP response codes and a standard error JSON envelope for all semantic failures:
          </p>

          <CodeBlock
            code={`{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Request validation failed.",
    "correlation_id": "req_01HMERCH00000000000001",
    "timestamp": "2026-11-17T18:30:00Z",
    "details": [
      { "field": "parcel.weight_kg", "issue": "pickup, dropoff and parcel.weight_kg > 0 are required" }
    ]
  }
}`}
            language="json"
            testId="error-envelope-sample"
          />

          <div className="space-y-2 mt-4">
            <h3 className="font-semibold text-sm">Status Code Reference</h3>
            <div className="m6-scroll" tabIndex={0} role="region" aria-label="Error codes table">
              <Table className="m6-table" aria-label="Error code reference">
                <TableHeader>
                  <TableRow>
                    <TableHead>HTTP</TableHead>
                    <TableHead>Error Code</TableHead>
                    <TableHead>Cause & Resolution</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell className="font-mono text-xs">400</TableCell>
                    <TableCell className="font-mono text-xs">AMBIGUOUS_CREDENTIALS</TableCell>
                    <TableCell>Sent both <code className="font-mono">X-API-Key</code> and Bearer token. Use only one.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">401</TableCell>
                    <TableCell className="font-mono text-xs">API_KEY_INVALID</TableCell>
                    <TableCell>API key hash or format is invalid. Check key prefix and secret.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">401</TableCell>
                    <TableCell className="font-mono text-xs">API_KEY_REVOKED / EXPIRED</TableCell>
                    <TableCell>Key has been revoked or expired. Create a new key in the API keys tab.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">403</TableCell>
                    <TableCell className="font-mono text-xs">MERCHANT_SCOPE_DENIED</TableCell>
                    <TableCell>API key lacks required scope (<code className="font-mono">parcels:create</code> or <code className="font-mono">parcels:read</code>).</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">403</TableCell>
                    <TableCell className="font-mono text-xs">MERCHANT_NOT_APPROVED</TableCell>
                    <TableCell>Merchant account is pending administrator review.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">409</TableCell>
                    <TableCell className="font-mono text-xs">IDEMPOTENCY_KEY_PAYLOAD_MISMATCH</TableCell>
                    <TableCell>An <code className="font-mono">Idempotency-Key</code> was reused with a different request payload.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">422</TableCell>
                    <TableCell className="font-mono text-xs">VALIDATION_FAILED</TableCell>
                    <TableCell>Missing required address fields or non-positive weight.</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell className="font-mono text-xs">400 / 422</TableCell>
                    <TableCell className="font-mono text-xs">SSRF_PROHIBITED_URL</TableCell>
                    <TableCell>Webhook URL points to a loopback, internal private, or cloud metadata IP.</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </div>
        </section>
      </div>
    </PortalLayout>
  );
};
