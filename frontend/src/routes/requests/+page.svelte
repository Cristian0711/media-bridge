<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { Button } from '$lib/components/ui/button';
  import RequestCard from '$lib/components/requests/request-card.svelte';
  import MediaRowSkeleton from '$lib/components/media/media-row-skeleton.svelte';
  import { getCached, isFresh } from '$lib/data/list-cache';
  import {
    loadRequestsListCached,
    requestsListCacheKey,
    REQUESTS_LIST_PAGE_SIZE,
  } from '$lib/data/requests-list-cache';
  import { createPaginatedList } from '$lib/data/paginated-list.svelte';
  import { preloadPostersFromRequestsResponse } from '$lib/utils/poster-preload';
  import { getAllRequests, getMyRequests } from '$lib/requests/list-api';
  import { requestsView, type RequestsView } from '$lib/data/requests-view';
  import type { PaginatedRequestsResponse, RequestRow } from '$lib/types/request';
  import { ApiError } from '$lib/api/client';
  import { requestsListVersion } from '$lib/sse/live-updates';
  import { infiniteScroll } from '$lib/utils/infinite-scroll';
  import { ClipboardList, Loader2, RefreshCw } from 'lucide-svelte';

  const pageSize = REQUESTS_LIST_PAGE_SIZE;

  const list = createPaginatedList<PaginatedRequestsResponse, RequestRow>({
    getCached: () => getCached<PaginatedRequestsResponse>(requestsListCacheKey($requestsView, 1)),
    isFresh: () => isFresh(requestsListCacheKey($requestsView, 1)),
    loadPageOne: (options) => loadRequestsListCached($requestsView, 1, options),
    fetchMore: (page) =>
      $requestsView === 'yours'
        ? getMyRequests({ page, pageSize })
        : getAllRequests({ page, pageSize }),
    toItems: (res) => res.requests ?? [],
    meta: (res) => ({ totalPages: res.total_pages, totalCount: res.total_count }),
    onApply: (res) => preloadPostersFromRequestsResponse(res),
    errorMessage: (e, kind) =>
      e instanceof ApiError ? e.message : kind === 'load' ? 'Failed to load requests' : 'Failed to load more',
  });

  let lastRequestsView: RequestsView | null = null;
  let sseHooked = false;

  $effect(() => {
    const _v = $requestsListVersion;
    if (!sseHooked) {
      sseHooked = true;
      return;
    }
    untrack(() => void list.reload());
  });

  $effect(() => {
    const view = $requestsView;
    const viewChanged = lastRequestsView !== null && lastRequestsView !== view;
    lastRequestsView = view;

    if (!viewChanged) return;

    untrack(() => {
      void list.reload();
    });
  });

  onMount(() => {
    void list.reload();
  });
</script>

<div class="-mx-6 flex flex-col">
  <div class="space-y-3 border-b border-border/30 px-3 pb-4 pt-5">
    <div class="rounded-lg border border-border/40 bg-card/50 px-3 py-2.5 text-sm">
      <div class="flex items-center justify-between">
        <span class="text-white/80">Total</span>
        <span class="font-semibold text-white">{list.totalCount}</span>
      </div>
    </div>

    <Button
      variant="outline"
      size="sm"
      class="h-9 w-full"
      disabled={list.loading}
      onclick={() => list.reload({ force: true })}
    >
      <RefreshCw class="mr-2 h-4 w-4 {list.loading ? 'animate-spin' : ''}" />
      Refresh
    </Button>

    {#if list.error}
      <p class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-300">{list.error}</p>
    {/if}
  </div>

  <div class="min-h-[1px] px-3 py-3">
    {#if list.loading && list.items.length === 0}
      <MediaRowSkeleton />
    {:else if list.items.length > 0}
      {#each list.items as req, index (req.id)}
        <RequestCard request={req} {index} showUsername={$requestsView === 'all'} />
      {/each}

      {#if list.hasMore}
        <div
          class="flex justify-center py-6"
          use:infiniteScroll={{ onLoadMore: list.loadMore }}
          aria-hidden="true"
        >
          {#if list.loadingMore}
            <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
          {/if}
        </div>
      {:else}
        <p class="py-4 text-center text-xs text-muted-foreground">End of list</p>
      {/if}
    {:else if !list.loading}
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
