import { withRequestReference } from '@/shared/lib/api-error';

export const apiBase = (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/$/, '');

export class PortalError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

type RequestOptions = {
  token?: string | null;
  method?: 'POST' | 'DELETE';
  body?: object;
  idempotencyKey?: string;
  signal?: AbortSignal;
  allowUnwrapped?: boolean;
  acceptPdf?: boolean;
};

export async function portalRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers({ Accept: options.acceptPdf ? 'application/pdf, application/json' : 'application/json' });
  if (options.token) headers.set('Authorization', `Bearer ${options.token}`);
  if (options.body) headers.set('Content-Type', 'application/json');
  if (options.idempotencyKey) headers.set('Idempotency-Key', options.idempotencyKey);
  const response = await fetch(apiBase + path, {
    method: options.method,
    body: options.body && JSON.stringify(options.body),
    signal: options.signal,
    cache: 'no-store',
    headers,
  });
  if (response.status === 204) return undefined as T;
  if (options.acceptPdf && response.ok && response.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() === 'application/pdf') {
    return await response.blob() as T;
  }
  if (response.headers.get('Content-Type')?.includes('text/html')) {
    throw new PortalError('The API returned a web page. Check that the preview points to the SwiftDrop API.', 502);
  }
  const envelope = await response.json().catch(() => null);
  if (!response.ok) {
    const message = envelope?.error?.message || `SwiftDrop could not complete the request (${response.status}).`;
    throw new PortalError(withRequestReference(message, response, envelope?.error?.correlation_id), response.status);
  }
  if (!envelope || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    if (options.allowUnwrapped && envelope && typeof envelope === 'object') return envelope as T;
    throw new PortalError('SwiftDrop returned an unexpected response. Check the API connection.', response.status);
  }
  return envelope.data as T;
}

export function errorMessage(error: unknown) {
  return error instanceof PortalError ? error.message : 'Could not reach SwiftDrop. Try again.';
}

export function statusLabel(status: string) {
  return status.toLowerCase().replace(/_/g, ' ');
}
