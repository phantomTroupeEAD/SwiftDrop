const errors: Record<string, string> = {
  INVALID_TRACKING_NUMBER: 'Enter a valid tracking number and try again.',
  TRACKING_NOT_FOUND: 'No parcel was found. Check the tracking number and try again.',
  COURIER_NOT_ON_SHIFT: 'That courier is no longer on shift. Choose another courier.',
};

export function friendlyApiError(response: Response, code?: unknown, fallback?: string): string {
  if (typeof code === 'string' && Object.prototype.hasOwnProperty.call(errors, code)) return errors[code];

  switch (response.status) {
    case 401: return fallback ?? 'Please sign in again to continue.';
    case 403: return 'You do not have permission to do that.';
    case 400:
    case 422: return fallback ?? 'Check the information you entered and try again.';
    case 404: return fallback ?? 'We could not find what you requested.';
    case 409: return fallback ?? 'This request conflicts with the current information. Refresh and try again.';
    case 429: return 'Please wait a moment and try again.';
    default: return fallback ?? 'SwiftDrop could not complete your request. Please try again.';
  }
}
