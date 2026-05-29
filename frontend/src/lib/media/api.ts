import { callApi } from '$lib/api/client';
import type { PaginatedMediaResponse } from '$lib/types/media-library';

type ListParams = {
  page?: number;
  pageSize?: number;
};

function queryString(params: ListParams & { q?: string }): string {
  const search = new URLSearchParams();
  if (params.page) search.set('page', String(params.page));
  if (params.pageSize) search.set('page_size', String(params.pageSize));
  if (params.q?.trim()) search.set('q', params.q.trim());
  const qs = search.toString();
  return qs ? `?${qs}` : '';
}

export async function getAllMedia(params: ListParams = {}): Promise<PaginatedMediaResponse> {
  return callApi<PaginatedMediaResponse>(`/media/list${queryString(params)}`);
}

export async function getMyMedia(params: ListParams = {}): Promise<PaginatedMediaResponse> {
  return callApi<PaginatedMediaResponse>(`/media/list/my${queryString(params)}`);
}

export async function searchAllMedia(
  query: string,
  params: ListParams = {},
): Promise<PaginatedMediaResponse> {
  return callApi<PaginatedMediaResponse>(`/media/search${queryString({ ...params, q: query })}`);
}

export async function searchMyMedia(
  query: string,
  params: ListParams = {},
): Promise<PaginatedMediaResponse> {
  return callApi<PaginatedMediaResponse>(`/media/search/my${queryString({ ...params, q: query })}`);
}
