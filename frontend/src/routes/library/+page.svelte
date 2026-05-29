<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import MediaLibraryItemCard from '$lib/components/media/media-library-item.svelte';
  import RemoveMediaDialog from '$lib/components/media/remove-media-dialog.svelte';
  import {
    getCached,
    isFresh,
  } from '$lib/data/list-cache';
  import {
    invalidateMediaListCache,
    loadMediaListCached,
    mediaListCacheKey,
    MEDIA_LIST_PAGE_SIZE,
  } from '$lib/data/media-list-cache';
  import { preloadPosterUrls } from '$lib/utils/poster-preload';
  import * as mediaApi from '$lib/media/api';
  import { formatSizeGB, toLibraryItem } from '$lib/media/map';
  import { libraryView } from '$lib/navigation/library-ui';
  import { removeMovie, removeShow } from '$lib/requests/api';
  import { mediaListVersion } from '$lib/sse/live-updates';
  import { infiniteScroll } from '$lib/utils/infinite-scroll';
  import type { LibraryView } from '$lib/navigation/library-ui';
  import type { MediaLibraryItem as LibraryItem, PaginatedMediaResponse } from '$lib/types/media-library';
  import { Film, Loader2, RefreshCw, Search, Users } from 'lucide-svelte';

  let items = $state<LibraryItem[]>([]);
  let loading = $state(false);
  let loadingMore = $state(false);
  let isSearching = $state(false);
  let error = $state('');
  let statusMessage = $state('');
  let searchQuery = $state('');
  let submittedQuery = $state('');

  let currentPage = $state(1);
  let fetchGeneration = 0;
  const pageSize = MEDIA_LIST_PAGE_SIZE;
  let totalPages = $state(1);
  let totalCount = $state(0);
  let totalSizeBytes = $state(0);

  const hasMore = $derived(items.length > 0 && currentPage < totalPages);

  let deleteOpen = $state(false);
  let itemToDelete = $state<LibraryItem | null>(null);
  let removing = $state(false);

  function applyPageOne(response: PaginatedMediaResponse) {
    items = response.media.map(toLibraryItem);
    totalPages = Math.max(1, response.total_pages);
    totalCount = response.total_count;
    totalSizeBytes = response.total_size_bytes ?? 0;
    currentPage = 1;
    preloadPosterUrls(items.map((i) => i.poster_url));
  }

  async function fetchMediaPage(page: number) {
    const view = $libraryView;
    const q = submittedQuery.trim();
    if (page === 1) {
      return loadMediaListCached(view, 1, q, { force: true });
    }
    const params = { page, pageSize };
    return q
      ? view === 'yours'
        ? mediaApi.searchMyMedia(q, params)
        : mediaApi.searchAllMedia(q, params)
      : view === 'yours'
        ? mediaApi.getMyMedia(params)
        : mediaApi.getAllMedia(params);
  }

  async function reload(options?: { force?: boolean }) {
    const generation = ++fetchGeneration;
    loadingMore = false;
    error = '';
    statusMessage = '';
    currentPage = 1;

    const view = $libraryView;
    const q = submittedQuery.trim();
    const key = mediaListCacheKey(view, 1, q);
    const cached = getCached<PaginatedMediaResponse>(key);

    if (cached && !options?.force) {
      applyPageOne(cached);
      loading = false;
    } else {
      loading = true;
    }

    const needsFetch = options?.force || !cached || !isFresh(key);
    if (!needsFetch) {
      isSearching = false;
      return;
    }

    try {
      const response = await loadMediaListCached(view, 1, q, { force: options?.force });
      if (generation !== fetchGeneration) return;
      applyPageOne(response);
    } catch (err) {
      if (generation !== fetchGeneration) return;
      if (!cached) {
        error = err instanceof Error ? err.message : 'Failed to load media';
        items = [];
        totalCount = 0;
        totalSizeBytes = 0;
      }
    } finally {
      if (generation === fetchGeneration) {
        loading = false;
        isSearching = false;
      }
    }
  }

  async function loadMore() {
    if (loading || loadingMore || !hasMore) return;

    const generation = fetchGeneration;
    const nextPage = currentPage + 1;
    loadingMore = true;

    try {
      const response = await fetchMediaPage(nextPage);
      if (generation !== fetchGeneration) return;

      const next = response.media.map(toLibraryItem);
      items = [...items, ...next];
      totalPages = Math.max(1, response.total_pages);
      totalCount = response.total_count;
      totalSizeBytes = response.total_size_bytes ?? 0;
      currentPage = nextPage;
    } catch (err) {
      if (generation !== fetchGeneration) return;
      error = err instanceof Error ? err.message : 'Failed to load more';
    } finally {
      if (generation === fetchGeneration) {
        loadingMore = false;
      }
    }
  }

  function queryFromInput(el?: HTMLInputElement | null): string {
    return (el?.value ?? searchQuery).trim();
  }

  async function runSearch(el?: HTMLInputElement | null) {
    const q = queryFromInput(el);
    searchQuery = q;
    submittedQuery = q;
    if (!q) {
      await reload({ force: true });
      return;
    }
    isSearching = true;
    await reload();
  }

  function onSearchKeydown(e: KeyboardEvent) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    void runSearch(e.currentTarget as HTMLInputElement);
  }

  function clearSearch() {
    searchQuery = '';
    submittedQuery = '';
    void reload();
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
      void reload({ force: true });
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to request removal';
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
    untrack(() => void reload());
  });

  $effect(() => {
    const view = $libraryView;
    const viewChanged = lastLibraryView !== null && lastLibraryView !== view;
    lastLibraryView = view;

    if (!viewChanged) return;

    untrack(() => {
      submittedQuery = '';
      searchQuery = '';
      void reload();
    });
  });

  onMount(() => {
    void reload();
  });
</script>

<div class="-mx-6 flex flex-col">
  <div class="space-y-3 border-b border-border/30 px-3 pb-4 pt-5">
    <div class="grid grid-cols-2 gap-2 text-sm">
      <div class="rounded-lg border border-border/40 bg-card/50 px-3 py-2.5">
        <div class="flex items-center justify-between">
          <span class="text-white/80">Total</span>
          <span class="font-semibold text-white">{totalCount}</span>
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
        disabled={loading || isSearching}
        aria-label={searchQuery.trim() ? 'Search' : 'Refresh list'}
      >
        {#if searchQuery.trim()}
          <Search class="h-4 w-4 {isSearching ? 'animate-spin' : ''}" />
        {:else}
          <RefreshCw class="h-4 w-4 {loading ? 'animate-spin' : ''}" />
        {/if}
      </Button>
    </form>

    {#if error}
      <p class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-300">{error}</p>
    {/if}
    {#if statusMessage}
      <p class="rounded-lg border border-green-500/40 bg-green-500/10 px-3 py-2 text-xs text-green-300">
        {statusMessage}
      </p>
    {/if}
  </div>

  <div class="min-h-[1px] px-3 py-3">
    {#if loading && items.length === 0}
      <p class="py-12 text-center text-sm text-muted-foreground">Loading…</p>
    {:else if items.length > 0}
      {#if submittedQuery}
        <p class="mb-3 text-xs text-muted-foreground">Results for “{submittedQuery}”</p>
      {/if}

      {#each items as item, index (item.id)}
        <MediaLibraryItemCard
          {item}
          {index}
          onRemove={$libraryView === 'yours' ? () => openRemove(item) : undefined}
        />
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
