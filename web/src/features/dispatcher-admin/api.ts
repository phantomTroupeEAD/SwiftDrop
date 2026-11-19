// Shared request helper for the admin console. The access token comes from AuthContext
// (memory only); nothing here stores a token anywhere (web/CLAUDE.md, invariant 3).

/** Fired on window when an admin API call answers 401, so the ops layout can refresh the session. */
export const UNAUTHORIZED_EVENT = 'swiftdrop:unauthorized';

export interface FieldIssue {
  field?: string;
  issue: string;
}

/** The standard error envelope, as an Error (api-conventions 2.3). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly correlationId: string;
  readonly details: FieldIssue[];

  constructor(status: number, code: string, message: string, correlationId = '', details: FieldIssue[] = []) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.correlationId = correlationId;
    this.details = details;
  }
}

interface ErrorBody {
  error?: {
    code?: string;
    message?: string;
    correlation_id?: string;
    details?: FieldIssue[];
  };
}

export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'PUT';

export async function request<T>(
  token: string | null,
  method: HttpMethod,
  url: string,
  body?: unknown,
  idempotencyKey?: string,
): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = `Bearer ${token}`;
  if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;

  let res: Response;
  try {
    res = await fetch(url, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  } catch {
    throw new ApiError(0, 'NETWORK_ERROR', 'Cannot reach the server. Check that Core API is running and try again.');
  }

  const text = await res.text();
  let parsed: unknown = null;
  try {
    parsed = text ? JSON.parse(text) : null;
  } catch {
    // Not JSON (for example a 502 page from a proxy).
  }

  if (!res.ok) {
    // An expired access token: tell the workspace so it can refresh the session (AuthContext).
    if (res.status === 401) window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT));
    const e = (parsed as ErrorBody | null)?.error;
    if (e?.code) {
      throw new ApiError(res.status, e.code, e.message ?? 'The request was refused.', e.correlation_id ?? '', e.details ?? []);
    }
    if (res.status === 502 || res.status === 503 || res.status === 504) {
      throw new ApiError(res.status, 'SERVICE_UNAVAILABLE', 'The service is not available right now. Try again shortly.');
    }
    throw new ApiError(res.status, 'HTTP_ERROR', `The request failed (HTTP ${res.status}).`);
  }
  return parsed as T;
}
