import EventSource from 'eventsource';
import { TEST_ENDPOINTS } from '../fixtures/testData';

export interface MilestoneEvent {
  trackingNumber: string;
  status: string;
  timestamp: string;
  location?: string;
  description?: string;
}

/**
 * Real-Time Server-Sent Events (SSE) Stream Listener
 * Connects to the Tracking API SSE stream and buffers milestones for assertion.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */
export class SSEStreamListener {
  private eventSource?: EventSource;
  private receivedEvents: MilestoneEvent[] = [];

  constructor(private baseUrl: string = TEST_ENDPOINTS.TRACKING_API_URL) {}

  /**
   * Subscribes to the live SSE milestone stream for a given tracking number.
   */
  subscribe(trackingNumber: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const streamUrl = `${this.baseUrl}/api/v1/tracking/${trackingNumber}/stream`;
      this.eventSource = new EventSource(streamUrl);

      this.eventSource.onopen = () => {
        resolve();
      };

      this.eventSource.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data) as MilestoneEvent;
          this.receivedEvents.push(parsed);
        } catch {
          // Ignore keepalive / malformed non-JSON pings
        }
      };

      this.eventSource.onerror = (err) => {
        // If initial connection fails, reject; if already connected, connection may close naturally
        if (this.receivedEvents.length === 0) {
          reject(err);
        }
      };
    });
  }

  /**
   * Waits for a specific status milestone to appear in the stream.
   */
  async waitForMilestone(expectedStatus: string, timeoutMs: number = 15000, pollIntervalMs: number = 500): Promise<MilestoneEvent> {
    const startTime = Date.now();

    while (Date.now() - startTime < timeoutMs) {
      const found = this.receivedEvents.find((e) => e.status.toUpperCase() === expectedStatus.toUpperCase());
      if (found) {
        return found;
      }
      await new Promise((resolve) => setTimeout(resolve, pollIntervalMs));
    }

    throw new Error(
      `Timeout waiting for milestone '${expectedStatus}' after ${timeoutMs}ms. Received events: ${JSON.stringify(this.receivedEvents)}`
    );
  }

  /**
   * Returns all events collected during this stream session.
   */
  getEvents(): MilestoneEvent[] {
    return [...this.receivedEvents];
  }

  /**
   * Closes the SSE connection and flushes buffers.
   */
  close(): void {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = undefined;
    }
  }
}
