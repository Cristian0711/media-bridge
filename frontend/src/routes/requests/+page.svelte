<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { Button } from '$lib/components/ui/button';
  import RequestCard from '$lib/components/requests/request-card.svelte';
  import { getCached, isFresh } from '$lib/data/list-cache';
  import {
    invalidateRequestsListCache,
    loadRequestsListCached,
    requestsListCacheKey,
    REQUESTS_LIST_PAGE_SIZE,
  } from '$lib/data/requests-list-cache';
  import { preloadPostersFromRequestsResponse } from '$lib/utils/poster-preload';
  import { getAllRequests, getMyRequests } from '$lib/requests/list-api';
  import { requestsView, type RequestsView } from '$lib/navigation/requests-ui';
  import type { PaginatedRequestsResponse, RequestRow } from '$lib/types/request';
  import { ApiError } from '$lib/api/client';
  import { requestsListVersion } from '$lib/sse/live-updates';
  import { infiniteScroll } from '$lib/utils/infinite-scroll';
  import { ClipboardList, Loader2, RefreshCw } from 'lucide-svelte';

  let requests = $state<RequestRow[]>([]);
  let loading = $state(false);
  let loadingMore = $state(false);
  let error = $state('');

  let currentPage = $state(1);
  const pageSize = REQUESTS_LIST_PAGE_SIZE;
  let totalPages = $state(1);
  let totalCount = $state(0);
  let fetchGeneration = 0;

  const hasMore = $derived(requests.length > 0 && currentPage < totalPages);

  function applyPageOne(response: PaginatedRequestsResponse) {
    requests = response.requests ?? [];
    totalPages = Math.max(1, response.total_pages);
    totalCount = response.total_count;
    currentPage = 1;
    preloadPostersFromRequestsResponse(response);
  }

  async function fetchRequestsPage(page: number) {
    const params = { page, pageSize };
    if (page === 1) {
      return loadRequestsListCached($requestsView, 1, { force: true });
    }
    return $requestsView === 'yours' ? await getMyRequests(params) : await getAllRequests(params);
  }

  async function reload(options?: { force?: boolean }) {
    const generation = ++fetchGeneration;
    loadingMore = false;
    error = '';
    currentPage = 1;

    const view = $requestsView;
    const key = requestsListCacheKey(view, 1);
    const cached = getCached<PaginatedRequestsResponse>(key);

    if (cached && !options?.force) {
      applyPageOne(cached);
      loading = false;
    } else {
      loading = true;
    }

    const needsFetch = options?.force || !cached || !isFresh(key);
    if (!needsFetch) return;

    try {
      const response = await loadRequestsListCached(view, 1, { force: options?.force });
      if (generation !== fetchGeneration) return;
      applyPageOne(response);
    } catch (e) {
      if (generation !== fetchGeneration) return;
      if (!cached) {
        requests = [];
        totalCount = 0;
        error = e instanceof ApiError ? e.message : 'Failed to load requests';
      }
    } finally {
      if (generation === fetchGeneration) {
        loading = false;
      }
    }
  }

  async function loadMore() {
    if (loading || loadingMore || !hasMore) return;

    const generation = fetchGeneration;
    const nextPage = currentPage + 1;
    loadingMore = true;

    try {
      const response = await fetchRequestsPage(nextPage);
      if (generation !== fetchGeneration) return;

      requests = [...requests, ...(response.requests ?? [])];
      totalPages = Math.max(1, response.total_pages);
      totalCount = response.total_count;
      currentPage = nextPage;
    } catch (e) {
      if (generation !== fetchGeneration) return;
      error = e instanceof ApiError ? e.message : 'Failed to load more';
    } finally {
      if (generation === fetchGeneration) {
        loadingMore = false;
      }
    }
  }

  let lastRequestsView: RequestsView | null = null;
  let sseHooked = false;

  $effect(() => {
    const _v = $requestsListVersion;
    if (!sseHooked) {
      sseHooked = true;
      return;
    }
    untrack(() => void reload());
  });

  $effect(() => {
    const view = $requestsView;
    const viewChanged = lastRequestsView !== null && lastRequestsView !== view;
    lastRequestsView = view;

    if (!viewChanged) return;

    untrack(() => {
      void reload();
    });
  });

  onMount(() => {
    void reload();
  });
</script>

<div class="-mx-6 flex flex-col">
  <div class="space-y-3 border-b border-border/30 px-3 pb-4 pt-5">
    <div class="rounded-lg border border-border/40 bg-card/50 px-3 py-2.5 text-sm">
      <div class="flex items-center justify-between">
        <span class="text-white/80">Total</span>
        <span class="font-semibold text-white">{totalCount}</span>
      </div>
    </div>

    <Button
      variant="outline"
      size="sm"
      class="h-9 w-full"
      disabled={loading}
      onclick={() => reload({ force: true })}
    >
      <RefreshCw class="mr-2 h-4 w-4 {loading ? 'animate-spin' : ''}" />
      Refresh
    </Button>

    {#if error}
      <p class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-300">{error}</p>
    {/if}
  </div>

  <div class="min-h-[1px] px-3 py-3">
    {#if loading && requests.length === 0}
      <p class="py-12 text-center text-sm text-muted-foreground">Loading…</p>
    {:else if requests.length > 0}
      {#each requests as req, index (req.id)}
        <RequestCard request={req} {index} showUsername={$requestsView === 'all'} />
      {/each}

      {#if hasMore}
        <div
          class="flex justify-center py-6"
          use:infiniteScroll={{ onLoadMore: loadMore }}
          aria-hidden="true"
        >
          {#if loadingMore}
            <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
          {/if}
        </div>
      {:else}
        <p class="py-4 text-center text-xs text-muted-foreground">End of list</p>
      {/if}
    {:else if !loading}
      <div class="py-12 text-center">
        <ClipboardList class="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
        <h3 class="mb-2 text-lg font-semibold text-white">No requests</h3>
        <p class="text-sm text-muted-foreground">
          {$requestsView === 'yours'
            ? 'Your download and remove jobs will appear here.'
            : 'No requests on the server yet.'}
        </p>
      </div>
    {/if}
  </div>
</div>
