import { callApi } from '$lib/api/client';
import { buildQuery } from '$lib/api/query';
import type { PaginatedMediaResponse } from '$lib/types/media-library';

type ListParams = {
  page?: number;
  pageSize?: number;
};

function queryString(params: ListParams & { q?: string }): string {
  return buildQuery({
    page: params.page || undefined,
    page_size: params.pageSize || undefined,
    q: params.q?.trim() || undefined,
  });
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
