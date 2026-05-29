import { goto } from '$app/navigation';
import { clearToken, getToken } from '$lib/auth/session';

export class ApiError extends Error {
  status: number;
  body: unknown;

  constructor(status: number, message: string, body: unknown = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

type CallApiOptions = RequestInit & {
  /** Attach Bearer token (default true). Set false for login/register. */
  auth?: boolean;
};

export async function callApi<T>(path: string, options: CallApiOptions = {}): Promise<T> {
  const { auth = true, ...init } = options;

  const headers = new Headers(init.headers);
  if (init.body != null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  if (auth) {
    const token = getToken();
    if (!token) {
      throw new ApiError(401, 'Not authenticated');
    }
    headers.set('Authorization', `Bearer ${token}`);
  }

  const res = await fetch(`/api/v1${path}`, {
    ...init,
    headers,
    credentials: 'include',
  });

  let body: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }

  if (!res.ok) {
    if (res.status === 401 && auth) {
      clearToken();
      if (typeof window !== 'undefined') {
        const redirect = encodeURIComponent(window.location.pathname + window.location.search);
        goto(`/login?redirect=${redirect}`);
      }
    }

    const message =
      typeof body === 'object' && body !== null && 'error' in body
        ? String((body as { error: string }).error)
        : res.statusText || 'Request failed';

    throw new ApiError(res.status, message, body);
  }

  return body as T;
}
