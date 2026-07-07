<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import MediaLibraryItemCard from '$lib/components/media/media-library-item.svelte';
  import MediaRowSkeleton from '$lib/components/media/media-row-skeleton.svelte';
  import RemoveMediaDialog from '$lib/components/media/remove-media-dialog.svelte';
  import { getCached, isFresh } from '$lib/data/list-cache';
  import {
    invalidateMediaListCache,
    loadMediaListCached,
    mediaListCacheKey,
    MEDIA_LIST_PAGE_SIZE,
  } from '$lib/data/media-list-cache';
  import { createPaginatedList } from '$lib/data/paginated-list.svelte';
  import { preloadPosterUrls } from '$lib/utils/poster-preload';
  import { POSTER_THUMB_WIDTH } from '$lib/utils/poster-url';
  import * as mediaApi from '$lib/media/api';
  import { formatSizeGB, toLibraryItem } from '$lib/media/map';
  import { libraryView } from '$lib/data/library-view';
  import { removeMovie, removeShow } from '$lib/requests/api';
  import { mediaListVersion } from '$lib/sse/live-updates';
  import { infiniteScroll } from '$lib/utils/infinite-scroll';
  import { setSearchInputFocused } from '$lib/navigation/search-ui.svelte';
  import type { LibraryView } from '$lib/data/library-view';
  import type { MediaLibraryItem as LibraryItem, PaginatedMediaResponse } from '$lib/types/media-library';
  import { Film, Loader2, RefreshCw, Search, Users } from 'lucide-svelte';

  let isSearching = $state(false);
  let statusMessage = $state('');
  let searchQuery = $state('');
  let submittedQuery = $state('');
  let totalSizeBytes = $state(0);

  const pageSize = MEDIA_LIST_PAGE_SIZE;

  let deleteOpen = $state(false);
  let itemToDelete = $state<LibraryItem | null>(null);
  let removing = $state(false);

  function fetchMediaPage(page: number) {
    const view = $libraryView;
    const q = submittedQuery.trim();
    const params = { page, pageSize };
    return q
      ? view === 'yours'
        ? mediaApi.searchMyMedia(q, params)
        : mediaApi.searchAllMedia(q, params)
      : view === 'yours'
        ? mediaApi.getMyMedia(params)
        : mediaApi.getAllMedia(params);
  }

  const list = createPaginatedList<PaginatedMediaResponse, LibraryItem>({
    getCached: () =>
      getCached<PaginatedMediaResponse>(mediaListCacheKey($libraryView, 1, submittedQuery.trim())),
    isFresh: () => isFresh(mediaListCacheKey($libraryView, 1, submittedQuery.trim())),
    loadPageOne: (options) =>
      loadMediaListCached($libraryView, 1, submittedQuery.trim(), options),
    fetchMore: (page) => fetchMediaPage(page),
    toItems: (res) => res.media.map(toLibraryItem),
    meta: (res) => ({ totalPages: res.total_pages, totalCount: res.total_count }),
    onApply: (res, items) => {
      totalSizeBytes = res.total_size_bytes ?? 0;
      preloadPosterUrls(items.map((i) => i.poster_url), { width: POSTER_THUMB_WIDTH });
    },
    onReloadStart: () => {
      statusMessage = '';
    },
    errorMessage: (err, kind) =>
      err instanceof Error ? err.message : kind === 'load' ? 'Failed to load media' : 'Failed to load more',
  });

  function queryFromInput(el?: HTMLInputElement | null): string {
    return (el?.value ?? searchQuery).trim();
  }

  async function runSearch(el?: HTMLInputElement | null) {
    const q = queryFromInput(el);
    searchQuery = q;
    submittedQuery = q;
    if (!q) {
      await list.reload({ force: true });
      isSearching = false;
      return;
    }
    isSearching = true;
    await list.reload();
    isSearching = false;
  }

  function onSearchKeydown(e: KeyboardEvent) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    void runSearch(e.currentTarget as HTMLInputElement);
  }

  function clearSearch() {
    searchQuery = '';
    submittedQuery = '';
    void list.reload();
  }

  function openRemove(item: LibraryItem) {
    itemToDelete = item;
    deleteOpen = true;
  }

  function onDeleteOpenChange(open: boolean) {
    deleteOpen = open;
    if (!open) itemToDelete = null;
  }

  async function confirmRemove() {
    if (!itemToDelete) return;
    removing = true;
    try {
      const ack =
        itemToDelete.type === 'movie'
          ? await removeMovie(itemToDelete.id)
          : await removeShow(itemToDelete.id);
      statusMessage = ack.message;
      invalidateMediaListCache();
      deleteOpen = false;
      itemToDelete = null;
      void list.reload({ force: true });
    } catch (err) {
      list.error = err instanceof Error ? err.message : 'Failed to request removal';
      deleteOpen = false;
      itemToDelete = null;
    } finally {
      removing = false;
    }
  }

  let lastLibraryView: LibraryView | null = null;
  let sseHooked = false;

  // Refresh when SSE invalidates the media cache (skip initial store read).
  $effect(() => {
    const _v = $mediaListVersion;
    if (!sseHooked) {
      sseHooked = true;
      return;
    }
    untrack(() => void list.reload());
  });

  $effect(() => {
    const view = $libraryView;
    const viewChanged = lastLibraryView !== null && lastLibraryView !== view;
    lastLibraryView = view;

    if (!viewChanged) return;

    untrack(() => {
      submittedQuery = '';
      searchQuery = '';
      void list.reload();
    });
  });

  onMount(() => {
    void list.reload();
  });
</script>

<div class="-mx-6 flex flex-col">
  <div class="space-y-3 border-b border-border/30 px-3 pb-4 pt-5">
    <div class="grid grid-cols-2 gap-2 text-sm">
      <div class="rounded-lg border border-border/40 bg-card/50 px-3 py-2.5">
        <div class="flex items-center justify-between">
          <span class="text-white/80">Total</span>
          <span class="font-semibold text-white">{list.totalCount}</span>
        </div>
      </div>
      <div class="rounded-lg border border-border/40 bg-card/50 px-3 py-2.5">
        <div class="flex items-center justify-between">
          <span class="text-white/80">Total size</span>
          <span class="font-semibold text-white">
            {formatSizeGB(totalSizeBytes) ?? '—'}
          </span>
        </div>
      </div>
    </div>

    <form
      class="flex items-center gap-2"
      onsubmit={(e) => {
        e.preventDefault();
        const input = e.currentTarget.querySelector<HTMLInputElement>('input');
        void runSearch(input);
      }}
    >
      <div class="relative flex-1">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="text"
          enterkeyhint="search"
          inputmode="search"
          autocomplete="off"
          placeholder="Search media..."
          class="h-9 pl-10"
          bind:value={searchQuery}
          onkeydown={onSearchKeydown}
          onfocus={() => setSearchInputFocused(true)}
          onblur={() => setSearchInputFocused(false)}
        />
      </div>
      {#if searchQuery.trim()}
        <Button variant="outline" size="sm" class="h-9" onclick={clearSearch}>Clear</Button>
      {/if}
      <Button
        type="submit"
        variant="outline"
        size="sm"
        class="h-9 w-9 p-0"
        disabled={list.loading || isSearching}
        aria-label={searchQuery.trim() ? 'Search' : 'Refresh list'}
      >
        {#if searchQuery.trim()}
          <Search class="h-4 w-4 {isSearching ? 'animate-spin' : ''}" />
        {:else}
          <RefreshCw class="h-4 w-4 {list.loading ? 'animate-spin' : ''}" />
        {/if}
      </Button>
    </form>

    {#if list.error}
      <p class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-300">{list.error}</p>
    {/if}
    {#if statusMessage}
      <p class="rounded-lg border border-green-500/40 bg-green-500/10 px-3 py-2 text-xs text-green-300">
        {statusMessage}
      </p>
    {/if}
  </div>

  <div class="min-h-[1px] px-3 py-3">
    {#if list.loading && list.items.length === 0}
      <MediaRowSkeleton />
    {:else if list.items.length > 0}
      {#if submittedQuery}
        <p class="mb-3 text-xs text-muted-foreground">Results for “{submittedQuery}”</p>
      {/if}

      {#each list.items as item, index (item.id)}
        <MediaLibraryItemCard
          {item}
          {index}
          onRemove={$libraryView === 'yours' ? () => openRemove(item) : undefined}
        />
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
        {#if submittedQuery}
          <Search class="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
          <h3 class="mb-2 text-lg font-semibold">No results</h3>
          <p class="text-sm text-muted-foreground">Nothing matches “{submittedQuery}”</p>
        {:else if $libraryView === 'yours'}
          <Film class="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
          <h3 class="mb-2 text-lg font-semibold">No media yet</h3>
          <p class="text-sm text-muted-foreground">Downloads you complete will appear here.</p>
        {:else}
          <Users class="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
          <h3 class="mb-2 text-lg font-semibold">No media found</h3>
          <p class="text-sm text-muted-foreground">The server library is empty.</p>
        {/if}
      </div>
    {/if}
  </div>
</div>

<RemoveMediaDialog
  open={deleteOpen}
  item={itemToDelete}
  {removing}
  onOpenChange={onDeleteOpenChange}
  onConfirm={confirmRemove}
/>
