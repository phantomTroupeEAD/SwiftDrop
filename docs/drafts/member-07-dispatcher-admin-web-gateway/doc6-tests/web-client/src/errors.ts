// Turns the standard error envelope (api-conventions 2.3) into one ApiError shape.
export interface ApiError {
  status: number; // 0 = network error
  code: string;
  message: string;
  correlationId?: string;
  fieldErrors: Record<string, string>;
}

interface Envelope {
  error?: { code?: string; message?: string; correlation_id?: string; details?: { field?: string; issue?: string }[] };
}

const SAFE_DEFAULTS: Record<number, string> = {
  401: 'Your session has ended. Please sign in again.',
  403: 'You do not have permission for this action.',
  404: 'This item no longer exists.',
  413: 'File is too large (maximum 10 MB).',
  429: 'Too many requests. Please wait and try again.',
};

export function toApiError(status: number, body: unknown): ApiError {
  const env = (typeof body === 'object' && body !== null ? body : {}) as Envelope;
  const e = env.error ?? {};
  const fieldErrors: Record<string, string> = {};
  for (const d of e.details ?? []) {
    if (d.field && d.issue) fieldErrors[d.field] = d.issue;
  }
  const fallback = status >= 500 || status === 0 ? 'Something went wrong. Please try again.' : SAFE_DEFAULTS[status] ?? 'Request failed.';
  return {
    status,
    code: e.code ?? (status === 0 ? 'NETWORK_ERROR' : `HTTP_${status}`),
    // Never show server text for 5xx: it could leak internals (INV-10).
    message: status >= 500 ? fallback : e.message ?? fallback,
    correlationId: e.correlation_id,
    fieldErrors,
  };
}
