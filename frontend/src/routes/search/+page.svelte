<script lang="ts">
  import { page } from '$app/state';
  import MediaCard from '$lib/components/media/media-card.svelte';
  import MediaRowSkeleton from '$lib/components/media/media-row-skeleton.svelte';
  import MediaActionHost, { type MediaRow } from '$lib/media/media-action-host.svelte';
  import { ApiError } from '$lib/api/client';
  import { searchMedia } from '$lib/search/api';
  import { toMediaItem } from '$lib/search/map';

  type SearchRow = NonNullable<ReturnType<typeof toMediaItem>>;

  let loading = $state(false);
  let loadingMore = $state(false);
  let error = $state('');
  let statusMessage = $state('');
  let results = $state<SearchRow[]>([]);
  let searchPage = $state(1);
  let totalPages = $state(1);
  let lastSearchedQuery = $state('');

  let mediaActions = $state<MediaActionHost | undefined>();
  let loadMoreEl: HTMLDivElement | undefined = $state();

  async function runSearch(q: string, append = false) {
    const trimmed = q.trim();
    if (!trimmed) {
      results = [];
      error = '';
      searchPage = 1;
      totalPages = 1;
      return;
    }

    if (append) {
      if (loadingMore || loading || searchPage >= totalPages) return;
      loadingMore = true;
    } else {
      loading = true;
      results = [];
      searchPage = 1;
      totalPages = 1;
    }
    error = '';
    if (!append) statusMessage = '';

    const pageNum = append ? searchPage + 1 : 1;

    try {
      const pageResult = await searchMedia(trimmed, pageNum);
      const rows = pageResult.results
        .map(toMediaItem)
        .filter((r): r is SearchRow => r !== null);
      if (append) {
        results = [...results, ...rows];
      } else {
        results = rows;
      }
      searchPage = pageResult.page;
      totalPages = pageResult.totalPages;
    } catch (e) {
      if (!append) results = [];
      error = e instanceof ApiError ? e.message : 'Search failed';
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  $effect(() => {
    const q = page.url.searchParams.get('q') ?? '';
    if (q === lastSearchedQuery) return;
    lastSearchedQuery = q;
    runSearch(q);
  });

  $effect(() => {
    const el = loadMoreEl;
    if (!el) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          const q = page.url.searchParams.get('q') ?? '';
          if (q.trim() && searchPage < totalPages) {
            runSearch(q, true);
          }
        }
      },
      { rootMargin: '200px' },
    );

    observer.observe(el);
    return () => observer.unobserve(el);
  });

  function rowKey(row: MediaRow, i: number): string {
    const tmdb = row.item.ids.tmdb;
    if (tmdb) return `${row.mediaType}-tmdb-${tmdb}`;
    return `${row.mediaType}-${row.item.title}-${i}`;
  }
</script>

{#if statusMessage}
  <p class="mb-3 rounded-lg border border-green-500/40 bg-green-500/10 px-3 py-2 text-xs text-green-300">
    {statusMessage}
  </p>
{/if}

{#if loading}
  <div class="-mx-1">
    <MediaRowSkeleton count={6} />
  </div>
{:else if error}
  <p class="text-sm text-red-400">{error}</p>
{:else if !page.url.searchParams.get('q')?.trim()}
  <p class="text-sm text-muted-foreground">Type a title and press return to search.</p>
{:else if results.length === 0}
  <p class="text-sm text-muted-foreground">No results found.</p>
{:else}
  <div class="-mx-1">
    {#each results as row, i (rowKey(row, i))}
      <MediaCard
        item={row.item}
        mediaType={row.mediaType}
        available={row.available}
        onSearch={() => mediaActions?.runIndexerSearch(row)}
        onDownload={() => mediaActions?.runDownload(row)}
      />
    {/each}
  </div>
  {#if searchPage < totalPages}
    <div bind:this={loadMoreEl} class="py-4 text-center text-xs text-muted-foreground">
      {#if loadingMore}
        Loading more…
      {:else}
        Scroll for more
      {/if}
    </div>
  {/if}
{/if}

<MediaActionHost bind:this={mediaActions} bind:statusMessage bind:error />
