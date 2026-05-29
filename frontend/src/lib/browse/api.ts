import { ApiError } from '$lib/api/client';
import { getToken } from '$lib/auth/session';
import type { SearchResult } from '$lib/types/search';

export type BrowseService = {
  id: string;
  name: string;
  logo_url: string;
};

export type BrowseListMeta = {
  id: string;
  title: string;
};

export type BrowsePage = {
  results: SearchResult[];
  page: number;
  totalPages: number;
};

export type BrowseListRow = BrowseListMeta & BrowsePage;

export type BrowseCatalog = {
  lists: BrowseListRow[];
};

async function authFetch(path: string): Promise<Response> {
  const token = getToken();
  if (!token) throw new ApiError(401, 'Not authenticated');
  return fetch(path, { headers: { Authorization: `Bearer ${token}` } });
}

export async function fetchBrowseServices(): Promise<BrowseService[]> {
  const res = await authFetch('/api/v1/browse/services');
  const body = await parseJson(res);
  if (!res.ok) throw apiErrorFrom(res, body);
  return body as BrowseService[];
}

export async function fetchBrowseServiceCatalog(serviceId: string): Promise<BrowseCatalog> {
  const res = await authFetch(
    `/api/v1/browse/services/${encodeURIComponent(serviceId)}/catalog`,
  );
  const body = await parseJson(res);
  if (!res.ok) throw apiErrorFrom(res, body);
  return body as BrowseCatalog;
}

export async function fetchBrowseGlobalCatalog(): Promise<BrowseCatalog> {
  const res = await authFetch('/api/v1/browse/global/catalog');
  const body = await parseJson(res);
  if (!res.ok) throw apiErrorFrom(res, body);
  return body as BrowseCatalog;
}

/** @deprecated Prefer fetchBrowseServiceCatalog */
export async function fetchBrowseServiceLists(serviceId: string): Promise<BrowseListMeta[]> {
  const res = await authFetch(`/api/v1/browse/services/${encodeURIComponent(serviceId)}/lists`);
  const body = await parseJson(res);
  if (!res.ok) throw apiErrorFrom(res, body);
  return body as BrowseListMeta[];
}

/** @deprecated Prefer fetchBrowseGlobalCatalog */
export async function fetchBrowseGlobalLists(): Promise<BrowseListMeta[]> {
  const res = await authFetch('/api/v1/browse/lists');
  const body = await parseJson(res);
  if (!res.ok) throw apiErrorFrom(res, body);
  return body as BrowseListMeta[];
}

export async function fetchBrowseList(id: string, page = 1): Promise<BrowsePage> {
  const params = new URLSearchParams({ page: String(page) });
  const res = await authFetch(`/api/v1/browse/${encodeURIComponent(id)}?${params.toString()}`);
  const body = await parseJson(res);
  if (!res.ok) throw apiErrorFrom(res, body);

  const pageNum = parseInt(res.headers.get('X-Search-Page') ?? String(page), 10) || page;
  const totalPages = parseInt(res.headers.get('X-Search-Total-Pages') ?? '1', 10) || 1;

  return {
    results: (body ?? []) as SearchResult[],
    page: pageNum,
    totalPages,
  };
}

async function parseJson(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function apiErrorFrom(res: Response, body: unknown): ApiError {
  const message =
    typeof body === 'object' && body !== null && 'error' in body
      ? String((body as { error: string }).error)
      : res.statusText || 'Request failed';
  return new ApiError(res.status, message, body);
}
