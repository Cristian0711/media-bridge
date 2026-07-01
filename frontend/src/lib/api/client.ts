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

export type ApiResponse<T> = {
  data: T;
  headers: Headers;
  status: number;
};

/**
 * Performs an authenticated `/api/v1` request and returns the parsed body
 * alongside the response headers and status. On 401 (when authed) it clears
 * the token and redirects to /login; on any non-2xx it throws ApiError.
 */
export async function callApiRaw<T>(
  path: string,
  options: CallApiOptions = {},
): Promise<ApiResponse<T>> {
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

  return { data: body as T, headers: res.headers, status: res.status };
}

export async function callApi<T>(path: string, options: CallApiOptions = {}): Promise<T> {
  const { data } = await callApiRaw<T>(path, options);
  return data;
}
