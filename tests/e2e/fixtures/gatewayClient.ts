import { APIRequestContext, request } from '@playwright/test';
import { TEST_ENDPOINTS, TEST_USERS } from './testData';

/**
 * Gateway & REST API Client Helper
 * Provides resilient polling, health check waiting, and JWT-authenticated seeding.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */
export class GatewayClient {
  private apiContext?: APIRequestContext;

  constructor(private baseUrl: string = TEST_ENDPOINTS.GATEWAY_URL) {}

  async getContext(): Promise<APIRequestContext> {
    if (!this.apiContext) {
      this.apiContext = await request.newContext({
        baseURL: this.baseUrl,
        extraHTTPHeaders: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
        },
      });
    }
    return this.apiContext;
  }

  /**
   * Polls a health endpoint until it returns 200 OK or times out.
   */
  async waitForHealth(endpoint: string, timeoutMs: number = 30000, intervalMs: number = 1000): Promise<boolean> {
    const ctx = await this.getContext();
    const startTime = Date.now();

    while (Date.now() - startTime < timeoutMs) {
      try {
        const response = await ctx.get(endpoint, { timeout: 2000 });
        if (response.ok()) {
          return true;
        }
      } catch {
        // Suppress network connection errors while service is booting
      }
      await new Promise((resolve) => setTimeout(resolve, intervalMs));
    }
    return false;
  }

  /**
   * Disposes the API request context.
   */
  async dispose(): Promise<void> {
    if (this.apiContext) {
      await this.apiContext.dispose();
      this.apiContext = undefined;
    }
  }
}
