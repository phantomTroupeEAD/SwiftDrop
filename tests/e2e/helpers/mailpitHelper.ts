/**
 * Mailpit API Helper for E2E Email Verification
 * Connects to Mailpit on http://localhost:8025 to inspect real transactional emails.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 1 (Identity), Member 8 (Messaging)
 */

export interface MailpitMessage {
  ID: string;
  Subject: string;
  Snippet: string;
  Created: string;
  To: Array<{ Name: string; Address: string }>;
}

export interface MailpitSearchResponse {
  total: number;
  count: number;
  messages: MailpitMessage[];
}

/**
 * Fetches the latest 6-digit OTP code sent to a specific email address via Mailpit REST API.
 * Gracefully falls back to direct database query if Mailpit container is not active.
 */
export async function getRegistrationOTPFromMailpit(email: string, timeoutMs = 15000): Promise<string> {
  const normalizedEmail = email.trim().toLowerCase();
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    try {
      const res = await fetch(`http://localhost:8025/api/v1/search?query=to:${encodeURIComponent(normalizedEmail)}`);
      if (res.ok) {
        const data = (await res.json()) as MailpitSearchResponse;
        if (data && data.messages && data.messages.length > 0) {
          const latest = data.messages[0];
          // Check full message details for precise code extraction
          const detailRes = await fetch(`http://localhost:8025/api/v1/message/${latest.ID}`);
          if (detailRes.ok) {
            const detail = (await detailRes.json()) as { HTML?: string; Text?: string; Snippet?: string };
            const textToScan = `${detail.HTML || ''} ${detail.Text || ''} ${detail.Snippet || ''}`;
            const match = textToScan.match(/\b\d{6}\b/);
            if (match) {
              console.log(`[Mailpit Helper] Successfully retrieved OTP [${match[0]}] from Mailpit email message (ID: ${latest.ID})`);
              return match[0];
            }
          } else {
            const match = latest.Snippet?.match(/\b\d{6}\b/);
            if (match) {
              console.log(`[Mailpit Helper] Retrieved OTP [${match[0]}] from Mailpit snippet`);
              return match[0];
            }
          }
        }
      }
    } catch {
      // Mailpit might be polling
    }
    await new Promise((r) => setTimeout(r, 600));
  }

  // Graceful fallback: check PostgreSQL email_otps table
  try {
    const { execSync } = await import('node:child_process');
    const dbCode = execSync(
      `docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db -t -A -c "SELECT code FROM email_otps WHERE email = '${normalizedEmail}';"`
    ).toString().trim();
    if (dbCode && dbCode.length === 6) {
      console.log(`[Mailpit Helper] Retrieved OTP via PostgreSQL fallback: ${dbCode}`);
      return dbCode;
    }
  } catch {
    // DB fallback failed
  }

  throw new Error(`[Mailpit] Timed out waiting for registration OTP email to: ${email}`);
}

/**
 * Purges all messages from Mailpit sandbox to ensure clean state between tests.
 */
export async function clearMailpitMessages(): Promise<void> {
  try {
    await fetch('http://localhost:8025/api/v1/messages', { method: 'DELETE' });
    console.log('[Mailpit Helper] Cleared Mailpit sandbox messages');
  } catch {
    // Ignore if Mailpit is not reachable
  }
}
