import { friendlyApiError } from '@/shared/lib/api-error';

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
  _retryCount?: number;
};

export async function portalRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers({ Accept: options.acceptPdf ? 'application/pdf, application/json' : 'application/json' });
  if (options.token) headers.set('Authorization', `Bearer ${options.token}`);
  if (options.body && !(options.body instanceof FormData)) headers.set('Content-Type', 'application/json');
  if (options.idempotencyKey) headers.set('Idempotency-Key', options.idempotencyKey);
  const response = await fetch(apiBase + path, {
    method: options.method,
    body: options.body instanceof FormData ? options.body : options.body && JSON.stringify(options.body),
    signal: options.signal,
    cache: 'no-store',
    headers,
  });
  if (response.status === 204) return undefined as T;
  if (options.acceptPdf && response.ok && response.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() === 'application/pdf') {
    return await response.blob() as T;
  }
  if (response.headers.get('Content-Type')?.includes('text/html')) {
    throw new PortalError('SwiftDrop is temporarily unavailable. Please try again later.', 502);
  }

  // Transparent session recovery on 401: attempt silent refresh once if refresh cookie exists
  if (response.status === 401 && !options._retryCount && !path.startsWith('/auth/')) {
    try {
      const refreshRes = await fetch(apiBase + '/auth/refresh', {
        method: 'POST',
        credentials: 'same-origin',
        signal: AbortSignal.timeout(8000),
      });
      if (refreshRes.ok) {
        const refreshData = await refreshRes.json();
        const newToken = refreshData?.data?.access_token;
        if (newToken) {
          if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('swiftdrop:token-refreshed', { detail: refreshData.data }));
          }
          return await portalRequest<T>(path, { ...options, token: newToken, _retryCount: 1 });
        }
      }
    } catch {
      // Refresh attempt failed, continue to throw PortalError below
    }
  }

  const envelope = await response.json().catch(() => null);
  if (!response.ok) {

    throw new PortalError(friendlyApiError(response, envelope?.error?.code), response.status);
  }
  if (!envelope || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    if (options.allowUnwrapped && envelope && typeof envelope === 'object') return envelope as T;
    throw new PortalError('SwiftDrop could not complete your request. Please try again.', response.status);
  }
  return envelope.data as T;
}

export function errorMessage(error: unknown) {
  return error instanceof PortalError ? error.message : 'Could not reach SwiftDrop. Try again.';
}

export function statusLabel(status: string) {
  return status.toLowerCase().replace(/_/g, ' ');
}
