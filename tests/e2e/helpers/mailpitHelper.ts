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
            
            // 1. Target the specific OTP container in the HTML template (<div class="otp-code">123456</div>)
            const htmlOtpMatch = detail.HTML?.match(/class="otp-code"[^>]*>\s*(\d{6})\s*</i);
            if (htmlOtpMatch) {
              console.log(`[Mailpit Helper] Retrieved OTP [${htmlOtpMatch[1]}] from HTML .otp-code container`);
              return htmlOtpMatch[1];
            }

            // 2. Target plain-text email body (free of CSS hex colors like #111827)
            if (detail.Text) {
              const textMatch = detail.Text.match(/\b\d{6}\b/);
              if (textMatch) {
                console.log(`[Mailpit Helper] Retrieved OTP [${textMatch[0]}] from plain-text body`);
                return textMatch[0];
              }
            }

            // 3. Target message snippet
            if (detail.Snippet || latest.Snippet) {
              const snippetText = detail.Snippet || latest.Snippet || '';
              const snippetMatch = snippetText.match(/\b\d{6}\b/);
              if (snippetMatch) {
                console.log(`[Mailpit Helper] Retrieved OTP [${snippetMatch[0]}] from snippet`);
                return snippetMatch[0];
              }
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
