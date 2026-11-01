import { toApiError, type ApiError } from './errors';
import { RefreshLock } from './refreshLock';

export interface TokenStore {
  get(): string | null;
  set(token: string | null): void;
}

export interface RequestOptions {
  idempotencyKey?: string;
}

type Fetch = (url: string, init: RequestInit) => Promise<Response>;

// Shared API client: adds the in-memory token, refreshes once on 401, repeats once,
// and throws ApiError for every failure.
export class ApiClient {
  private readonly lock: RefreshLock;
  constructor(
    private readonly baseUrl: string,
    private readonly tokens: TokenStore,
    private readonly fetchFn: Fetch,
    private readonly onSignedOut: () => void,
  ) {
    this.lock = new RefreshLock(() => this.refresh());
  }

  get<T>(path: string): Promise<T> {
    return this.send<T>('GET', path, undefined, {});
  }

  post<T>(path: string, body: unknown, opts: RequestOptions = {}): Promise<T> {
    return this.send<T>('POST', path, body, opts);
  }

  private async send<T>(method: string, path: string, body: unknown, opts: RequestOptions, retried = false): Promise<T> {
    let res: Response;
    try {
      res = await this.fetchFn(this.baseUrl + path, {
        method,
        headers: this.headers(opts, body !== undefined),
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw toApiError(0, null);
    }
    if (res.status === 401 && !retried && !path.startsWith('/auth/')) {
      const token = await this.lock.run();
      if (token) return this.send<T>(method, path, body, opts, true);
      this.tokens.set(null);
      this.onSignedOut();
    }
    const json = await res.json().catch(() => null);
    if (!res.ok) throw toApiError(res.status, json) as ApiError;
    return (json as { data: T }).data;
  }

  private headers(opts: RequestOptions, hasBody: boolean): Record<string, string> {
    const h: Record<string, string> = {};
    const t = this.tokens.get();
    if (t) h['Authorization'] = `Bearer ${t}`;
    if (hasBody) h['Content-Type'] = 'application/json';
    if (opts.idempotencyKey) h['Idempotency-Key'] = opts.idempotencyKey;
    return h;
  }

  private async refresh(): Promise<string | null> {
    try {
      const res = await this.fetchFn(this.baseUrl + '/auth/refresh', { method: 'POST', credentials: 'include' });
      if (!res.ok) return null;
      const json = (await res.json()) as { data: { access_token: string } };
      this.tokens.set(json.data.access_token);
      return json.data.access_token;
    } catch {
      return null;
    }
  }
}
