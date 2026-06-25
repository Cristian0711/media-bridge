import { callApi } from '$lib/api/client';
import { buildQuery } from '$lib/api/query';
import type { PaginatedRequestsResponse } from '$lib/types/request';

type ListParams = {
  page?: number;
  pageSize?: number;
};

function queryString(params: ListParams): string {
  return buildQuery({
    page: params.page || undefined,
    page_size: params.pageSize || undefined,
  });
}

export async function getAllRequests(params: ListParams = {}): Promise<PaginatedRequestsResponse> {
  return callApi<PaginatedRequestsResponse>(`/requests${queryString(params)}`);
}

export async function getMyRequests(params: ListParams = {}): Promise<PaginatedRequestsResponse> {
  return callApi<PaginatedRequestsResponse>(`/requests/my${queryString(params)}`);
}
