import { callApi } from '$lib/api/client';
import type { PaginatedRequestsResponse } from '$lib/types/request';

type ListParams = {
  page?: number;
  pageSize?: number;
};

function queryString(params: ListParams): string {
  const search = new URLSearchParams();
  if (params.page) search.set('page', String(params.page));
  if (params.pageSize) search.set('page_size', String(params.pageSize));
  const qs = search.toString();
  return qs ? `?${qs}` : '';
}

export async function getAllRequests(params: ListParams = {}): Promise<PaginatedRequestsResponse> {
  return callApi<PaginatedRequestsResponse>(`/requests${queryString(params)}`);
}

export async function getMyRequests(params: ListParams = {}): Promise<PaginatedRequestsResponse> {
  return callApi<PaginatedRequestsResponse>(`/requests/my${queryString(params)}`);
}
