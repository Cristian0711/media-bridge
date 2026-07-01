import { callApi, callApiRaw } from '$lib/api/client';
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

export async function fetchBrowseServices(): Promise<BrowseService[]> {
  return callApi<BrowseService[]>('/browse/services');
}

export async function fetchBrowseServiceCatalog(serviceId: string): Promise<BrowseCatalog> {
  return callApi<BrowseCatalog>(`/browse/services/${encodeURIComponent(serviceId)}/catalog`);
}

export async function fetchBrowseGlobalCatalog(): Promise<BrowseCatalog> {
  return callApi<BrowseCatalog>('/browse/global/catalog');
}

export async function fetchBrowseList(id: string, page = 1): Promise<BrowsePage> {
  const params = new URLSearchParams({ page: String(page) });
  const { data, headers } = await callApiRaw<SearchResult[] | null>(
    `/browse/${encodeURIComponent(id)}?${params.toString()}`,
  );

  const pageNum = parseInt(headers.get('X-Search-Page') ?? String(page), 10) || page;
  const totalPages = parseInt(headers.get('X-Search-Total-Pages') ?? '1', 10) || 1;

  return {
    results: data ?? [],
    page: pageNum,
    totalPages,
  };
}
