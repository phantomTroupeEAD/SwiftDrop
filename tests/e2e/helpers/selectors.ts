/**
 * SwiftDrop Centralized UI Selector Contract Registry
 * Authoritative source of data-testid attributes across all 5 user portals.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 6 (Customer & Merchant UI), Member 7 (Dispatcher & Admin UI)
 *
 * INVARIANT: Never use CSS classes, DOM hierarchy, or user-facing text strings
 * for core operational assertions. Always reference selectors from this file.
 */

export const SELECTORS = {
  // ---------------------------------------------------------------------------
  // 1. Common Authentication & Layout
  // ---------------------------------------------------------------------------
  AUTH: {
    INPUT_EMAIL: 'login-email-input',
    INPUT_PASSWORD: 'login-password-input',
    BTN_LOGIN: 'login-submit-btn',
    BTN_LOGOUT: 'btn-logout',
    USER_PROFILE_MENU: 'user-profile-menu',
    AUTH_ERROR_ALERT: 'auth-error-alert',
  },

  // ---------------------------------------------------------------------------
  // 2. Customer Portal — Booking & Fee Estimation
  // ---------------------------------------------------------------------------
  CUSTOMER: {
    // Fee Estimation & Booking Wizard
    BOOKING_WIZARD: 'booking-wizard',
    INPUT_PICKUP_POSTAL: 'booking-pickup-postal-input',
    INPUT_DROPOFF_POSTAL: 'booking-dropoff-postal-input',
    INPUT_PICKUP_NAME: 'booking-pickup-name-input',
    INPUT_PICKUP_PHONE: 'booking-pickup-phone-input',
    INPUT_PICKUP_ADDRESS: 'booking-pickup-address-input',
    INPUT_PICKUP_CITY: 'booking-pickup-city-input',
    INPUT_DROPOFF_NAME: 'booking-dropoff-name-input',
    INPUT_DROPOFF_PHONE: 'booking-dropoff-phone-input',
    INPUT_DROPOFF_ADDRESS: 'booking-dropoff-address-input',
    INPUT_DROPOFF_CITY: 'booking-dropoff-city-input',
    INPUT_WEIGHT: 'booking-weight-input',
    INPUT_LENGTH: 'booking-length-input',
    INPUT_WIDTH: 'booking-width-input',
    INPUT_HEIGHT: 'booking-height-input',
    INPUT_DESCRIPTION: 'booking-description-input',
    SELECT_SERVICE: 'booking-service-select',
    BTN_GET_QUOTE: 'booking-next-btn',
    BTN_NEXT: 'booking-next-btn',
    BTN_BACK: 'booking-back-btn',
    QUOTE_PRICE_CENTS: 'quote-price-cents',
    BTN_SUBMIT_BOOKING: 'booking-submit-btn',
    BOOKING_CONFIRMATION: 'booking-confirmation',
    BOOKING_CONFIRMATION_TRACKING_NO: 'booking-confirmation-tracking-no',
    BTN_RECEIPT: 'booking-receipt-btn',
    BOOKING_RECEIPT: 'booking-receipt',
    PARCEL_STATUS_BADGE: 'parcel-status-badge',
  },

  // ---------------------------------------------------------------------------
  // 3. Dispatcher Board — Routing & Courier Assignment
  // ---------------------------------------------------------------------------
  DISPATCHER: {
    DISPATCHER_BOARD: 'dispatcher-board',
    COURIER_SELECT: 'dispatcher-courier-select',
    ASSIGN_BTN: 'dispatcher-assign-btn',
    RETRY_BTN: 'dispatcher-retry-btn',
    DISPATCH_UNASSIGNED_TABLE: 'dispatch-unassigned-table',
    ROW_PARCEL_ITEM: 'row-parcel-item',
    SELECT_COURIER_DROPDOWN: 'select-courier-dropdown',
    BTN_ASSIGN_COURIER: 'btn-assign-courier',
    BADGE_ASSIGNMENT_STATUS: 'badge-assignment-status',
    FILTER_ZONE_DROPDOWN: 'filter-zone-dropdown',
    COURIER_CAPACITY_METER: 'courier-capacity-meter',
  },

  // ---------------------------------------------------------------------------
  // 4. Courier Mobile PWA — Milestone & POD Submission
  // ---------------------------------------------------------------------------
  COURIER: {
    COURIER_TASK_LIST: 'courier-task-list',
    TASK_ITEM: 'courier-task-item',
    BTN_ACCEPT_JOB: 'btn-accept-job',
    BTN_START_PICKUP: 'btn-start-pickup',
    BTN_CONFIRM_PICKUP: 'btn-confirm-pickup',
    BTN_START_DELIVERY: 'btn-start-delivery',
    BTN_MARK_DELIVERED: 'btn-mark-delivered',
    
    // Digital Proof of Delivery (POD)
    POD_MODAL: 'pod-submission-modal',
    INPUT_POD_RECIPIENT_NAME: 'input-pod-recipient-name',
    INPUT_POD_SIGNATURE_CANVAS: 'input-pod-signature-canvas',
    INPUT_POD_PHOTO_UPLOAD: 'input-pod-photo-upload',
    BTN_SUBMIT_POD: 'btn-submit-pod',
    POD_SUCCESS_CONFIRMATION: 'pod-success-confirmation',
  },

  // ---------------------------------------------------------------------------
  // 5. Merchant Portal — Tenants, API Keys & Webhooks
  // ---------------------------------------------------------------------------
  MERCHANT: {
    TENANT_PROFILE_BADGE: 'tenant-profile-badge',
    API_KEY_DISPLAY: 'merchant-api-key-display',
    BTN_REGENERATE_API_KEY: 'btn-regenerate-api-key',
    INPUT_WEBHOOK_URL: 'input-webhook-url',
    INPUT_WEBHOOK_SECRET: 'input-webhook-secret',
    BTN_SAVE_WEBHOOK: 'btn-save-webhook',
    WEBHOOK_STATUS_INDICATOR: 'webhook-status-indicator',
    MERCHANT_PARCEL_TABLE: 'merchant-parcel-table',
    BTN_BULK_UPLOAD: 'btn-bulk-upload',
  },

  // ---------------------------------------------------------------------------
  // 6. Admin Portal — Zones, Tariffs & Audit Logs
  // ---------------------------------------------------------------------------
  ADMIN: {
    ADMIN_ZONE_TABLE: 'admin-zone-table',
    BTN_CREATE_ZONE: 'btn-create-zone',
    ADMIN_TARIFF_TABLE: 'admin-tariff-table',
    INPUT_BASE_TARIFF: 'input-base-tariff',
    BTN_UPDATE_TARIFF: 'btn-update-tariff',
    ADMIN_AUDIT_LOG_TABLE: 'admin-audit-log-table',
    AUDIT_LOG_ROW: 'audit-log-row',
    USER_MANAGEMENT_TABLE: 'user-management-table',
  },

  // ---------------------------------------------------------------------------
  // 7. Tracking & Real-Time SSE
  // ---------------------------------------------------------------------------
  TRACKING: {
    SEARCH_INPUT: 'tracking-search-input',
    SEARCH_BTN: 'tracking-search-btn',
    STATUS_BADGE: 'tracking-status-badge',
    TIMELINE: 'tracking-timeline',
    INPUT_TRACKING_NUMBER: 'input-tracking-number',
    BTN_TRACK_PARCEL: 'btn-track-parcel',
    TRACKING_RESULT_CARD: 'tracking-result-card',
    MILESTONE_TIMELINE: 'milestone-timeline',
    MILESTONE_STATUS_BADGE: 'milestone-status-badge',
    MILESTONE_ITEM: 'milestone-item',
    SSE_CONNECTION_INDICATOR: 'sse-connection-indicator',
    LAST_UPDATED_TIMESTAMP: 'last-updated-timestamp',
  },

  // ---------------------------------------------------------------------------
  // 8. AI Assistant Drawer & FastMCP Chat
  // ---------------------------------------------------------------------------
  AI_ASSISTANT: {
    PANEL: 'assistant-panel',
    PROMPT_INPUT: 'assistant-prompt-input',
    SEND_BTN: 'assistant-send-btn',
    CHAT_TOGGLE_BUTTON: 'ai-chat-toggle-btn',
    CHAT_DRAWER: 'ai-chat-drawer',
    INPUT_AI_QUERY: 'input-ai-query',
    BTN_SEND_AI_QUERY: 'btn-send-ai-query',
    CHAT_MESSAGE_LIST: 'ai-chat-message-list',
    CHAT_MESSAGE_BUBBLE: 'ai-chat-message-bubble',
    AI_GROUNDED_PARCEL_CARD: 'ai-grounded-parcel-card',
    AI_ACTION_BUTTON_BOOK: 'ai-action-btn-book',
    AI_ACTION_BUTTON_TRACK: 'ai-action-btn-track',
  },
} as const;

/**
 * Type-safe helper to format a data-testid attribute for Playwright queries.
 * Example: `page.locator(testId(SELECTORS.CUSTOMER.BTN_SUBMIT_BOOKING))`
 */
export function testId(id: string): string {
  return `[data-testid="${id}"]`;
}
